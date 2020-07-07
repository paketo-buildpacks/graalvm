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
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/pavel-v-chernykh/keystore-go"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	"github.com/paketo-buildpacks/graalvm/graalvm"
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
		dep := libpak.BuildpackDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "0bdf99b069660cf63f33d26edf11cc1aea5aba9126ca06a86b8f1cfd610352b1",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := graalvm.NewJDK(dep, nil, dc, filepath.Join("testdata", "test-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Logger = bard.NewLogger(ioutil.Discard)

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]interface{})["cacerts-sha256"]).
			To(Equal("04846f73d9d0421c60076fd02bad7f0a81a3f11a028d653b0de53290e41dcead"))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Build).To(BeTrue())
		Expect(layer.Cache).To(BeTrue())
		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
		Expect(layer.BuildEnvironment["JAVA_HOME.override"]).To(Equal(layer.Path))
		Expect(layer.BuildEnvironment["JDK_HOME.override"]).To(Equal(layer.Path))
	})

	it("contributes native image to JDK", func() {
		executor.On("Execute", mock.Anything).Return(nil)

		jdkDep := libpak.BuildpackDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "0bdf99b069660cf63f33d26edf11cc1aea5aba9126ca06a86b8f1cfd610352b1",
		}
		niDep := &libpak.BuildpackDependency{
			URI:    "https://localhost/stub-native-image.jar",
			SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := graalvm.NewJDK(jdkDep, niDep, dc, filepath.Join("testdata", "test-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Logger = bard.NewLogger(ioutil.Discard)
		j.Executor = executor

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		executor := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(executor.Command).To(Equal(filepath.Join(layer.Path, "bin", "gu")))
		Expect(executor.Args).To(Equal([]string{"install", "--local-file", filepath.Join("testdata", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "stub-native-image.jar")}))
		Expect(executor.Dir).To(Equal(layer.Path))
	})

	it("updates before Java 9 certificates", func() {
		dep := libpak.BuildpackDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jdk-8.tar.gz",
			SHA256:  "c16f4c171399b7cdfd7ad83e76934eef8e1776578e803b82164956d68a0f3aa7",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := libjvm.NewJDK(dep, dc, filepath.Join("testdata", "test-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Logger = bard.NewLogger(ioutil.Discard)

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		in, err := os.Open(filepath.Join(layer.Path, "jre", "lib", "security", "cacerts"))
		Expect(err).NotTo(HaveOccurred())
		defer in.Close()

		ks, err := keystore.Decode(in, []byte("changeit"))
		Expect(err).NotTo(HaveOccurred())

		Expect(ks).To(HaveLen(2))
	})

	it("updates after Java 9 certificates", func() {
		dep := libpak.BuildpackDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "0bdf99b069660cf63f33d26edf11cc1aea5aba9126ca06a86b8f1cfd610352b1",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		j, err := libjvm.NewJDK(dep, dc, filepath.Join("testdata", "test-certificates.crt"), &libcnb.BuildpackPlan{})
		Expect(err).NotTo(HaveOccurred())
		j.Logger = bard.NewLogger(ioutil.Discard)

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = j.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		in, err := os.Open(filepath.Join(layer.Path, "lib", "security", "cacerts"))
		Expect(err).NotTo(HaveOccurred())
		defer in.Close()

		ks, err := keystore.Decode(in, []byte("changeit"))
		Expect(err).NotTo(HaveOccurred())

		Expect(ks).To(HaveLen(2))
	})

}
