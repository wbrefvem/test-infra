FROM ubuntu:18.04

RUN apt-get -y update
RUN apt-get -y upgrade
RUN apt-get -y install pkg-config zip g++ zlib1g-dev unzip python python3 curl git
RUN curl -O -L https://github.com/bazelbuild/bazel/releases/download/0.22.0/bazel-0.22.0-installer-linux-x86_64.sh
RUN /bin/bash bazel-0.22.0-installer-linux-x86_64.sh

RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
RUN apt-get -y install apt-transport-https ca-certificates
RUN curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -
RUN apt-get -y update && apt-get -y install google-cloud-sdk

CMD ["bazel", "build", "//prow/..."]
