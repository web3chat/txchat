#!/bin/bash
# 编译 comet / logic，并输出进度
# 用法：bash builder.sh <targetDir> [os_arch] ["server list"]
work_dir=$(
  cd "$(dirname "$0")" || exit
  pwd
)/../..

targetDir=$1
targetOsArch=$2
serviceList=${3:-"comet logic"}

if [ "${targetDir}" = "" ]; then
  echo "target directory is empty"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: 未找到 go 命令，请先安装 Go 并配置 PATH" >&2
  exit 127
fi

progress_script="${work_dir}/script/compose/progress.sh"
if [ -f "${progress_script}" ]; then
  # shellcheck source=/dev/null
  source "${progress_script}"
fi

golangOsArch() {
  if [ "$1" = "" ]; then
    envOsArch=$(go version | awk '{ print $4 }')
    targetOS=$(echo "${envOsArch}" | awk -F '/' '{ print $1 }')
    targetARCH=$(echo "${envOsArch}" | awk -F '/' '{ print $2 }')
  else
    targetOS=$(echo "$1" | awk -F '_' '{ print $1 }')
    targetARCH=$(echo "$1" | awk -F '_' '{ print $2 }')
  fi

  if [ "${targetOS}" = "" ] || [ "${targetARCH}" = "" ]; then
    echo "error: got empty os or arch type!"
    exit 1
  fi
}

ldflagsOfInject() {
  versionFilePath="main"
  # 只匹配 v* 根标签，避免子模块 tag（如 im/v0.1.0）带斜杠污染镜像 tag。
  projectVersion=$(git describe --abbrev=8 --tags --match 'v*' 2>/dev/null || git rev-parse --short=8 HEAD)
  goVersion=$(go version | awk '{ print $3 }')
  gitCommit=$(git rev-parse --short=8 HEAD)
  buildTime=$(date "+%Y-%m-%d %H:%M:%S %Z")
  osArch="${targetOS}/${targetARCH}"

  ldflags="\
  -X '${versionFilePath}.projectVersion=${projectVersion}' \
  -X '${versionFilePath}.goVersion=${goVersion}' \
  -X '${versionFilePath}.gitCommit=${gitCommit}' \
  -X '${versionFilePath}.buildTime=${buildTime}' \
  -X '${versionFilePath}.osArch=${osArch}' \
  -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"
}

exportGOEnv() {
  export GOOS=${targetOS}
  export GOARCH=${targetARCH}
  export CGO_ENABLED=0
  export GO111MODULE=on
  export GOPROXY=https://goproxy.cn,direct
  export GOSUMDB='sum.golang.google.cn'
}

golangOsArch "${targetOsArch}"
ldflagsOfInject
exportGOEnv

buildTargetDir="${work_dir}/${targetDir}/"
mkdir -p "${buildTargetDir}"

# shellcheck disable=SC2086
set -- ${serviceList}
total_local=$#
idx=0
offset="${PROGRESS_OFFSET:-0}"
total_global="${PROGRESS_TOTAL:-${total_local}}"

for sName in ${serviceList}; do
  idx=$((idx + 1))
  cur=$((offset + idx))
  case "${sName}" in
    comet)
      confSrc="${work_dir}/comet/conf/comet.toml"
      mainSrc="${work_dir}/comet/cmd/main.go"
      ;;
    logic)
      confSrc="${work_dir}/logic/conf/logic.toml"
      mainSrc="${work_dir}/logic/cmd/main.go"
      ;;
    *)
      echo "ERROR: unknown service: ${sName}" >&2
      exit 1
      ;;
  esac

  echo "┌ start building ${sName} service"
  cp "${confSrc}" "${buildTargetDir}/${sName}.toml" || exit 1
  if declare -F run_with_progress >/dev/null 2>&1; then
    run_with_progress "${cur}" "${total_global}" "compile" "${sName}" -- \
      go build -ldflags "${ldflags}" -v -o "${buildTargetDir}/${sName}" "${mainSrc}"
  else
    go build -ldflags "${ldflags}" -v -o "${buildTargetDir}/${sName}" "${mainSrc}" || exit 1
  fi
  echo "└ building ${sName} service success"
done
