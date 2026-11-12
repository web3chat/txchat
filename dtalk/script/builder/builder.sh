#!/bin/bash
work_dir=$(
  cd "$(dirname "$0")" || exit
  pwd
)/../..
# import .sh files

# $1:build target dir; $2:build target architecture(like linux_amd64); $3:service list
targetDir=$1
targetOsArch=$2
serviceList=$3

if [ "${targetDir}" = "" ]; then
  echo "target directory is empty"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: 未找到 go 命令，请先安装 Go 并配置 PATH" >&2
  exit 127
fi

golangOsArch() {
  if [ "$1" = "" ]; then
    envOsArch=$(go version | awk '{ print $4 }')
    targetOS=$(echo "${envOsArch}" | awk -F '/' '{ print $1 }')
    targetARCH=$(echo "${envOsArch}" | awk -F '/' '{ print $2 }')
  else
    # set target os and arch
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
  #projectVersion=$(git describe --abbrev=0 --tags)
  # 只匹配 v* 根标签，避免子模块 tag（如 dtalk/v0.1.0）带斜杠污染镜像 tag。
  projectVersion=$(git describe --abbrev=8 --tags --match 'v*' 2>/dev/null || git rev-parse --short=8 HEAD)
  goVersion=$(go version | awk '{ print $3 }')
  #goVersion=$(shell go version | awk '{ print $$3 }')
  gitCommit=$(git rev-parse --short=8 HEAD)
  #buildTime=`date +%Y%m%d`
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

# $1:service name; $2: config root path; $3: service main filename
buildService() {
  serviceName="$1"
  configPath="$2"
  serviceMain="$3"

  echo "┌ start building ${serviceName} service"
  cp "${configPath}/${serviceName}.toml" "${buildTargetDir}/${serviceName}.toml" || exit
  go build -ldflags "${ldflags}" -o "${buildTargetDir}/${serviceName}" "${serviceMain}" || exit
  echo "└ building ${serviceName} service success"
}

progress_script="${work_dir}/script/compose/progress.sh"
# shellcheck disable=SC1090
if [ -f "${progress_script}" ]; then
  # shellcheck source=/dev/null
  source "${progress_script}"
fi
# shellcheck disable=SC2086
set -- ${serviceList}
total_local=$#
idx=0
offset="${DTALK_PROGRESS_OFFSET:-0}"
total_global="${DTALK_PROGRESS_TOTAL:-${total_local}}"

mkdir -p "${buildTargetDir}"
for sName in ${serviceList} ; do
    idx=$((idx + 1))
    cur=$((offset + idx))
    case ${sName} in
      "gateway")
        configPath="${work_dir}/gateway/api/v1/etc"
        serviceMain="${work_dir}/gateway/api/v1/gateway.go"
        ;;
      "answer"|"pusher"|"store")
        configPath="${work_dir}/service/record/${sName}/config"
        serviceMain="${work_dir}/service/record/${sName}/cmd/main.go"
        ;;
      "chatclean")
        configPath="${work_dir}/service/${sName}/etc"
        serviceMain="${work_dir}/service/${sName}/cmd/main.go"
        ;;
      "federation")
        configPath="${work_dir}/service/${sName}/config"
        serviceMain="${work_dir}/service/${sName}/cmd/main.go"
        ;;
      *)
        configPath="${work_dir}/service/${sName}/config"
        serviceMain="${work_dir}/service/${sName}/cmd/main.go"
        ;;
    esac

    echo "┌ start building ${sName} service"
    cp "${configPath}/${sName}.toml" "${buildTargetDir}/${sName}.toml" || exit 1
    if declare -F run_with_progress >/dev/null 2>&1; then
      run_with_progress "${cur}" "${total_global}" "compile" "${sName}" -- \
        go build -ldflags "${ldflags}" -o "${buildTargetDir}/${sName}" "${serviceMain}"
    else
      go build -ldflags "${ldflags}" -o "${buildTargetDir}/${sName}" "${serviceMain}" || exit 1
    fi
    echo "└ building ${sName} service success"
done