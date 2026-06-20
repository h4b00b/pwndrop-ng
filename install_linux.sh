#!/bin/bash -e
# Auto-detect the host architecture and pull the matching release artifact.
# Override with ARCH=amd64 / ARCH=arm64 to force a specific download.

case "${ARCH:-$(uname -m)}" in
    amd64|x86_64)   ARCH=amd64 ;;
    arm64|aarch64)  ARCH=arm64 ;;
    *)
        echo "*** unsupported architecture: ${ARCH:-$(uname -m)}" >&2
        echo "*** set ARCH=amd64 or ARCH=arm64 to force a download" >&2
        exit 1
        ;;
esac

FILENAME=pwndrop-linux-${ARCH}
mkdir -p ${FILENAME}
cd ${FILENAME}
echo "*** downloading pwndrop (${ARCH})."
wget https://github.com/kgretzky/pwndrop/releases/latest/download/${FILENAME}.tar.gz
echo "*** unpacking."
tar zxvf ${FILENAME}.tar.gz
cd pwndrop
chmod 700 pwndrop
echo "*** stopping pwndrop."
./pwndrop stop
echo "*** installing."
./pwndrop install
./pwndrop start
./pwndrop status
echo "*** cleaning up."
cd ../..
rm -rf ${FILENAME}/
