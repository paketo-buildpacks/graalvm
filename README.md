# `gcr.io/paketo-buildpacks/graalvm`

The Paketo GraalVM Buildpack is a Cloud Native Buildpack that provides the GraalVM implementations of the JDK and GraalVM [Native Image builder][native-image].

This buildpack is designed to work in collaboration with other buildpacks which request contributions of JREs, JDKs, or Native Image builder.

## Behavior

This buildpack will participate if any of the following conditions are met

* Another buildpack requires `jdk`
* Another buildpack requires `jre`
* Another buildpack requires `native-image-builder`

The buildpack will do the following if a JDK is requested:

* Contributes a JDK to a layer marked `build` and `cache` with all commands on `$PATH`
* Contributes `$JAVA_HOME` configured to the build layer
* Contributes `$JDK_HOME` configure to the build layer

The buildpack will do the following if `native-image-builder` is requested:
* Contribute a JDK (see above)
* Installs the Native Image Substrate VM into the JDK
* Prevents the JRE from being installed, even if requested

The buildpack will do the following if a JRE is requested:

* Contributes a JDK to a layer with all commands on `$PATH` (GraalVM does not distribute a standalone JRE)
* Contributes `$JAVA_HOME` configured to the layer
* Contributes `-XX:ActiveProcessorCount` to the layer
* Contributes `-XX:+ExitOnOutOfMemoryError` to the layer
* Contributes `$MALLOC_ARENA_MAX` to the layer
* Disables JVM DNS caching if link-local DNS is available
* If `metadata.build = true`
  * Marks layer as `build` and `cache`
* If `metadata.launch = true`
  * Marks layer as `launch`
* Contributes Memory Calculator to a layer marked `launch`
* Contributes Heap Dump helper to a layer marked `launch`

## Configuration

| Environment Variable | Description
| -------------------- | -----------
| `$BP_JVM_VERSION` | Configure a specific JVM version (e.g. `8`, `11`, `14`).  The buildpack will download JDK and Native Image Substrate VM assets that are compatible with this version of the JVM specification.  Since the buildpack only ships a single version of each supported line, updates to the buildpack can change the exact version of the JDK or JRE.  In order to hold the JDK and JRE versions stable, the buildpack version itself must be stable.<p/><p/>Buildpack releases (and the dependency versions for each release) can be found [here][bpv].  Few users will use this buildpack directly, instead consuming a language buildpack like `paketo-buildpacks/java` who's releases (and the individual buildpack versions and dependency versions for each release) can be found [here](https://github.com/paketo-buildpacks/java/releases).  Finally, some users will will consume builders like `paketobuildpacks/builder:base` who's releases can be found [here](https://hub.docker.com/r/paketobuildpacks/builder/tags?page=1&name=base).  To determine the individual buildpack versions and dependency versions for each builder release use the [`pack inspect-builder <image>`](https://buildpacks.io/docs/reference/pack/pack_inspect-builder/) functionality.
| `$BPL_JVM_HEAD_ROOM` | Configure the percentage of headroom the memory calculator will allocated.  Defaults to `0`.
| `$BPL_JVM_LOADED_CLASS_COUNT` | Configure the number of classes that will be loaded at runtime.  Defaults to 35% of the number of classes.
| `$BPL_JVM_THREAD_COUNT` | Configure the number of user threads at runtime.  Defaults to `250`.
| `$BPL_HEAP_DUMP_PATH` | Configure the location for writing heap dumps in the event of an OutOfMemoryError exception. Defaults to ``, which disables writing heap dumps. The path set must be writable by the JVM process.
| `$JAVA_TOOL_OPTIONS` | Configure the JVM launch flags

[bpv]: https://github.com/paketo-buildpacks/graalvm/releases

## Bindings

The buildpack optionally accepts the following bindings:

### Type: `dependency-mapping`

|Key                   | Value   | Description
|----------------------|---------|------------
|`<dependency-digest>` | `<uri>` | If needed, the buildpack will fetch the dependency with digest `<dependency-digest>` from `<uri>`

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0
[native-image]: https://www.graalvm.org/reference-manual/native-image/

