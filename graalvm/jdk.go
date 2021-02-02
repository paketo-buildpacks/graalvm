/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package graalvm

import (
	"fmt"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/effect"
)

type JDK struct {
	CertificateLoader     libjvm.CertificateLoader
	DependencyCache       libpak.DependencyCache
	Executor              effect.Executor
	JDKDependency         libpak.BuildpackDependency
	LayerContributor      libpak.LayerContributor
	Logger                bard.Logger
	NativeImageDependency *libpak.BuildpackDependency
}

func NewJDK(jdkDependency libpak.BuildpackDependency, nativeImageDependency *libpak.BuildpackDependency, cache libpak.DependencyCache, certificateLoader libjvm.CertificateLoader) (JDK, []libcnb.BOMEntry, error) {
	dependencies := []libpak.BuildpackDependency{jdkDependency}

	if nativeImageDependency != nil {
		dependencies = append(dependencies, *nativeImageDependency)
	}

	expected := map[string]interface{}{"dependencies": dependencies}

	if md, err := certificateLoader.Metadata(); err != nil {
		return JDK{}, nil, fmt.Errorf("unable to generate certificate loader metadata")
	} else {
		for k, v := range md {
			expected[k] = v
		}
	}

	contributor := libpak.NewLayerContributor(
		bard.FormatIdentity(jdkDependency.Name, jdkDependency.Version),
		expected,
		libcnb.LayerTypes{
			Build: true,
			Cache: true,
		},
	)
	j := JDK{
		CertificateLoader:     certificateLoader,
		DependencyCache:       cache,
		Executor:              effect.NewExecutor(),
		JDKDependency:         jdkDependency,
		LayerContributor:      contributor,
		NativeImageDependency: nativeImageDependency,
	}

	var bomEntries []libcnb.BOMEntry
	entry := jdkDependency.AsBOMEntry()
	entry.Metadata["layer"] = j.Name()
	entry.Build = true
	bomEntries = append(bomEntries, entry)

	if nativeImageDependency != nil {
		entry := nativeImageDependency.AsBOMEntry()
		entry.Metadata["layer"] = j.Name()
		entry.Launch = true
		entry.Build = true
		bomEntries = append(bomEntries, entry)
	}

	return j, bomEntries, nil
}

func (j JDK) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	j.LayerContributor.Logger = j.Logger

	return j.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {
		artifact, err := j.DependencyCache.Artifact(j.JDKDependency)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to get dependency %s\n%w", j.JDKDependency.ID, err)
		}
		defer artifact.Close()

		j.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.ExtractTarGz(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand JDK\n%w", err)
		}

		layer.BuildEnvironment.Override("JAVA_HOME", layer.Path)
		layer.BuildEnvironment.Override("JDK_HOME", layer.Path)

		var keyStorePath string
		if libjvm.IsBeforeJava9(j.JDKDependency.Version) {
			keyStorePath = filepath.Join(layer.Path, "jre", "lib", "security", "cacerts")
		} else {
			keyStorePath = filepath.Join(layer.Path, "lib", "security", "cacerts")
		}

		if err := j.CertificateLoader.Load(keyStorePath, "changeit"); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to load certificates\n%w", err)
		}

		if j.NativeImageDependency != nil {
			j.Logger.Header(color.BlueString("%s %s", j.NativeImageDependency.Name, j.NativeImageDependency.Version))

			artifact, err := j.DependencyCache.Artifact(*j.NativeImageDependency)
			if err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to get dependency %s\n%w", j.NativeImageDependency.ID, err)
			}
			defer artifact.Close()

			j.Logger.Body("Installing substrate VM")

			if err := j.Executor.Execute(effect.Execution{
				Command: filepath.Join(layer.Path, "bin", "gu"),
				Args:    []string{"install", "--local-file", artifact.Name()},
				Dir:     layer.Path,
				Stdout:  j.Logger.InfoWriter(),
				Stderr:  j.Logger.InfoWriter(),
			}); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to run gu install\n%w", err)
			}
		}

		return layer, nil
	})
}

func (JDK) Name() string {
	return "jdk"
}
