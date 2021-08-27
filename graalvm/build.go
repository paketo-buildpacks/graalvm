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

	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	pr := libpak.PlanEntryResolver{Plan: context.Plan}

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
	}

	dc, err := libpak.NewDependencyCache(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency cache\n%w", err)
	}
	dc.Logger = b.Logger

	cl := libjvm.NewCertificateLoader()
	cl.Logger = b.Logger.BodyWriter()

	v, _ := cr.Resolve("BP_JVM_VERSION")
	var nativeImage bool

	if _, ok, err := pr.Resolve(PlanEntryJDK); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve jdk plan entry\n%w", err)
	} else if ok {
		jdkDependency, err := dr.Resolve(PlanEntryJDK, v)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
		}

		var nativeImageDependency *libpak.BuildpackDependency
		if _, nativeImage, err = pr.Resolve(PlanEntryNativeImageBuilder); err != nil {
			return libcnb.BuildResult{}, fmt.Errorf(
				"unable to resolve %s plan entry\n%w",
				PlanEntryNativeImageBuilder,
				err,
			)
		}
		if nativeImage {
			dep, err := dr.Resolve("native-image-svm", v)
			if err != nil {
				return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
			}
			nativeImageDependency = &dep
		}

		jdk, be, err := NewJDK(jdkDependency, nativeImageDependency, dc, cl)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create jdk\n%w", err)
		}

		jdk.Logger = b.Logger
		result.Layers = append(result.Layers, jdk)
		result.BOM.Entries = append(result.BOM.Entries, be...)
	}

	if e, ok, err := pr.Resolve(PlanEntryJRE); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve jre plan entry\n%w", err)
	} else if ok && !nativeImage {
		dt := libjvm.JREType
		depJRE, err := dr.Resolve("jre", v)

		if libpak.IsNoValidDependencies(err) {
			warn := color.New(color.FgYellow, color.Bold)
			b.Logger.Header(warn.Sprint("No valid JRE available, providing matching JDK instead. Using a JDK at runtime has security implications."))

			dt = libjvm.JDKType
			depJRE, err = dr.Resolve("jdk", v)
		}

		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
		}

		jre, be, err := libjvm.NewJRE(context.Application.Path, depJRE, dc, dt, cl, e.Metadata)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create jre\n%w", err)
		}

		jre.Logger = b.Logger
		result.Layers = append(result.Layers, jre)
		result.BOM.Entries = append(result.BOM.Entries, be)

		if libjvm.IsLaunchContribution(e.Metadata) {
			helpers := []string{"active-processor-count", "java-opts", "jvm-heap", "link-local-dns", "memory-calculator",
				"openssl-certificate-loader", "security-providers-configurer"}

			if libjvm.IsBeforeJava9(depJRE.Version) {
				helpers = append(helpers, "security-providers-classpath-8")
			} else {
				helpers = append(helpers, "security-providers-classpath-9")
			}

			h, be := libpak.NewHelperLayer(context.Buildpack, helpers...)
			h.Logger = b.Logger
			result.Layers = append(result.Layers, h)
			result.BOM.Entries = append(result.BOM.Entries, be)

			jsp := libjvm.NewJavaSecurityProperties(context.Buildpack.Info)
			jsp.Logger = b.Logger
			result.Layers = append(result.Layers, jsp)
		}
	}

	return result, nil
}
