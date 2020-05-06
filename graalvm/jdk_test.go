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

package graalvm_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/graalvm/graalvm"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"
)

func testJDK(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx      libcnb.BuildContext
		executor *mocks.Executor
	)

	it.Before(func() {
		var err error

		ctx.Layers.Path, err = ioutil.TempDir("", "jdk-layers")
		Expect(err).NotTo(HaveOccurred())

		executor = &mocks.Executor{}
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes JDK", func() {
		executor.On("Execute", mock.Anything).Return(nil)

		dep := libpak.BuildpackDependency{
			URI:    "https://localhost/stub-jdk.tar.gz",
			SHA256: "c03a11439c65028091a9328024ce97d6b90ca220f4e2c90c5d8a65e3ba7c1ef2",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := graalvm.NewJDK(dep, nil, dc, filepath.Join("testdata", "ca-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Executor = executor

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]interface{})["cacerts-sha256"]).
			To(Equal("d3f4f98b2670973c120d3cc4f4991d260646af116c621d194a0df2533d9dcb83"))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Build).To(BeTrue())
		Expect(layer.Cache).To(BeTrue())
		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
		Expect(layer.BuildEnvironment["JAVA_HOME.override"]).To(Equal(layer.Path))
		Expect(layer.BuildEnvironment["JDK_HOME.override"]).To(Equal(layer.Path))
		Expect(len(executor.Calls)).To(Equal(133))
	})

	it("contributes native image to JDK", func() {
		executor.On("Execute", mock.Anything).Return(nil)

		jdkDep := libpak.BuildpackDependency{
			URI:    "https://localhost/stub-jdk.tar.gz",
			SHA256: "c03a11439c65028091a9328024ce97d6b90ca220f4e2c90c5d8a65e3ba7c1ef2",
		}
		niDep := &libpak.BuildpackDependency{
			URI:    "https://localhost/stub-native-image.jar",
			SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := graalvm.NewJDK(jdkDep, niDep, dc, filepath.Join("testdata", "ca-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Executor = executor

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		executor := executor.Calls[133].Arguments[0].(effect.Execution)
		Expect(executor.Command).To(Equal(filepath.Join(layer.Path, "bin", "gu")))
		Expect(executor.Args).To(Equal([]string{"install", "--local-file", filepath.Join("testdata", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "stub-native-image.jar")}))
		Expect(executor.Dir).To(Equal(layer.Path))
	})

	it("updates before Java 9 certificates", func() {
		executor.On("Execute", mock.Anything).Return(nil)

		dep := libpak.BuildpackDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jdk.tar.gz",
			SHA256:  "e8a33e8283efc5a039d90b0ef61ce4613ad01544316caaf307b28da46d49a108",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := libjvm.NewJDK(dep, dc, filepath.Join("testdata", "ca-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Executor = executor

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		execution := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(execution.Command).To(Equal(filepath.Join(layer.Path, "bin", "keytool")))
		Expect(execution.Args[6]).To(Equal(filepath.Join(layer.Path, "jre", "lib", "security", "cacerts")))
	})

	it("updates after Java 9 certificates", func() {
		executor.On("Execute", mock.Anything).Return(nil)

		dep := libpak.BuildpackDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk.tar.gz",
			SHA256:  "00abd23ea484b935aa7565a0bbc1b6d56b4875a80f6a5b3b4d36ff7b3ac01a1c",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := libjvm.NewJDK(dep, dc, filepath.Join("testdata", "ca-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Executor = executor

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		execution := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(execution.Command).To(Equal(filepath.Join(layer.Path, "bin", "keytool")))
		Expect(execution.Args[6]).To(Equal(filepath.Join(layer.Path, "lib", "security", "cacerts")))
	})

}
