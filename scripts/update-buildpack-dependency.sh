sha256() {
  if [[ "${DEPENDENCY}" == "jdk" ]]; then
    shasum -a 256 "${ROOT}"/dependency/graalvm-ce-java*-linux-amd64-*.tar.gz | cut -f 1 -d ' '
  elif [[ "${DEPENDENCY}" == "native-image-svm" ]]; then
    shasum -a 256 "${ROOT}"/dependency/native-image-installable-svm-java*-linux-amd64-*.jar | cut -f 1 -d ' '
  elif [[ "${DEPENDENCY}" == "jvmkill" ]]; then
    shasum -a 256 "${ROOT}"/dependency/jvmkill-*.so | cut -f 1 -d ' '
  elif [[ "${DEPENDENCY}" == "memory-calculator" ]]; then
    shasum -a 256 "${ROOT}"/dependency/memory-calculator-*.tgz | cut -f 1 -d ' '
  else
    cat "${ROOT}"/dependency/sha256
  fi
}

uri() {
  if [[ "${DEPENDENCY}" == "jdk" ]]; then
    echo "https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-$(cat "${ROOT}"/dependency/version)/$(basename "${ROOT}"/dependency/graalvm-ce-java*-linux-amd64-*.tar.gz)"
  elif [[ "${DEPENDENCY}" == "native-image-svm" ]]; then
    echo "https://github.com/graalvm/graalvm-ce-builds/releases/download/vm-$(cat "${ROOT}"/dependency/version)/$(basename "${ROOT}"/dependency/native-image-installable-svm-java*-linux-amd64-*.jar)"
  elif [[ "${DEPENDENCY}" == "jvmkill" ]]; then
    echo "https://github.com/cloudfoundry/jvmkill/releases/download/v$(cat "${ROOT}"/dependency/version)/$(basename "${ROOT}"/dependency/jvmkill-*.so)"
  elif [[ "${DEPENDENCY}" == "memory-calculator" ]]; then
    echo "https://github.com/cloudfoundry/java-buildpack-memory-calculator/releases/download/v$(cat "${ROOT}"/dependency/version)/$(basename "${ROOT}"/dependency/memory-calculator-*.tgz)"
  else
    cat "${ROOT}"/dependency/uri
  fi
}

version() {
  if [[ "${DEPENDENCY}" == "jdk" || "${DEPENDENCY}" == "native-image-svm" ]]; then
    local PATTERN='JAVA_VERSION="([0-9]+)\.?([0-9]*)\.?([0-9]*)_?([0-9]+)?"'

    if [[ $(tar xOz --wildcards --no-recursion -f "${ROOT}"/dependency/graalvm-ce-java*-linux-amd64-*.tar.gz 'graalvm-ce-java*/release' | grep JAVA_VERSION) =~ ${PATTERN} ]]; then
      if  [[ "${BASH_REMATCH[1]}" = "1" ]]; then
        echo "${BASH_REMATCH[2]}.${BASH_REMATCH[3]}.${BASH_REMATCH[4]}"
        return
      else
        echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
        return
      fi
    else
      echo "version is not semver" 1>&2
      exit 1
    fi
  else
    local PATTERN="([0-9]+)\.?([0-9]*)\.?([0-9]*)(.*)"

    if [[ $(cat "${ROOT}"/dependency/version) =~ ${PATTERN} ]]; then
      echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
      return
    else
      echo "version is not semver" 1>&2
      exit 1
    fi
  fi
}
