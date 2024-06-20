#!/bin/bash
#
# Copyright © 2024 HAMi Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
set -e

export VERSION="v2.3.11"
export GOLANG_IMAGE="golang:1.21-bullseye"
export NVIDIA_IMAGE="nvidia/cuda:12.4.0-devel-ubuntu20.04"
export DEST_DIR="/usr/local"

IMAGE=${IMAGE-"aicphub/hami"}
platform="all" #arm64,amd64,all

function go_build() {
  [[ -z "$J" ]] && J=$(nproc | awk '{print int(($0 + 1)/ 2)}')
  make -j$J
}


function pre_build_image()
{
	image=$1

	# check repository image
	repositoryImageExists=$(curl --silent -f --head -lL https://hub.docker.com/v2/repositories/$image/tags/$VERSION/ > /dev/null && echo "success" || echo "failed")

	# check repository image
	if [ "$repositoryImageExists" == "success" ]; then
		echo "docker image $image:$VERSION exists on dockerhub, skiping ... "
		exit 255
	fi

	echo "pre_build_image success!"
	return 0
}

function build_image() {
	image=$1
	platform=$2

	build_cmd="docker buildx build --no-cache "
	if [ $platform == "all" ]; then
		builder_exists=$(docker buildx ls | awk '{if ($1=="multi-platform") print $1}')
		if [ "$builder_exists" ]; then
			docker buildx rm multi-platform
		fi
		# create a new builder instance
		docker buildx create --use --bootstrap --name multi-platform --platform=linux/amd64,linux/arm64 > /dev/null

		build_cmd="$build_cmd --push --platform linux/amd64,linux/arm64"
	else
		build_cmd="$build_cmd -o type=docker --platform linux/${platform}"
	fi

	cmd="$build_cmd --build-arg VERSION=${VERSION} --build-arg GOLANG_IMAGE=${GOLANG_IMAGE} --build-arg NVIDIA_IMAGE=${NVIDIA_IMAGE} --build-arg DEST_DIR=${DEST_DIR} -t ${IMAGE}:${VERSION} -f docker/Dockerfile ."
	echo $cmd
	$cmd
}

go_build

pre_build_image "$IMAGE"
build_image "$IMAGE" "$platform"
