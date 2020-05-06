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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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
	Certificates          string
	DependencyCache       libpak.DependencyCache
	Executor              effect.Executor
	JDKDependency         libpak.BuildpackDependency
	LayerContributor      libpak.LayerContributor
	Logger                bard.Logger
	NativeImageDependency *libpak.BuildpackDependency
}

func NewJDK(jdkDependency libpak.BuildpackDependency, nativeImageDependency *libpak.BuildpackDependency,
	cache libpak.DependencyCache, certificates string, plan *libcnb.BuildpackPlan) (JDK, error) {

	dependencies := []libpak.BuildpackDependency{jdkDependency}
	plan.Entries = append(plan.Entries, jdkDependency.AsBuildpackPlanEntry())

	if nativeImageDependency != nil {
		dependencies = append(dependencies, *nativeImageDependency)
		plan.Entries = append(plan.Entries, nativeImageDependency.AsBuildpackPlanEntry())
	}

	expected := map[string]interface{}{"dependencies": dependencies}

	in, err := os.Open(certificates)
	if err != nil && !os.IsNotExist(err) {
		return JDK{}, fmt.Errorf("unable to open file %s\n%w", certificates, err)
	} else if err == nil {
		defer in.Close()

		s := sha256.New()
		if _, err := io.Copy(s, in); err != nil {
			return JDK{}, fmt.Errorf("unable to hash file %s\n%w", certificates, err)
		}
		expected["cacerts-sha256"] = hex.EncodeToString(s.Sum(nil))
	}

	return JDK{
		Certificates:          certificates,
		DependencyCache:       cache,
		Executor:              effect.NewExecutor(),
		JDKDependency:         jdkDependency,
		LayerContributor:      libpak.NewLayerContributor(bard.FormatIdentity(jdkDependency.Name, jdkDependency.Version), expected),
		NativeImageDependency: nativeImageDependency,
	}, nil
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

		var destination string
		if libjvm.IsBeforeJava9(j.JDKDependency.Version) {
			destination = filepath.Join(layer.Path, "jre", "lib", "security", "cacerts")
		} else {
			destination = filepath.Join(layer.Path, "lib", "security", "cacerts")
		}

		c := libjvm.CertificateLoader{
			KeyTool:         filepath.Join(layer.Path, "bin", "keytool"),
			SourcePath:      j.Certificates,
			DestinationPath: destination,
			Executor:        j.Executor,
			Logger:          j.Logger,
		}

		if err := c.Load(); err != nil {
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

		layer.Build = true
		layer.Cache = true
		return layer, nil
	})
}

func (JDK) Name() string {
	return "jdk"
}
