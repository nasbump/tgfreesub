#!/bin/sh

appname=$(basename $(pwd))

outdir="build"
[ -d ${outdir} ] || mkdir -p ${outdir}

outname="${appname}"

last_md5=$(md5sum ${outdir}/${outname} 2>/dev/null | awk '{print $1}')

export GOPROXY="https://goproxy.cn,direct"
go mod tidy
go mod vendor
CGO_ENABLED=0 go build -ldflags "-s -w" -v -o ${outdir}/${outname}

echo
new_md5=$(md5sum ${outdir}/${outname} 2>/dev/null)
if echo ${new_md5} | grep ${last_md5} >/dev/null; then
    echo "============build failed: not changed ========="
    echo "last_md5=${last_md5}"
    echo "new_md5=${new_md5}"
    exit 1
fi

echo "============build done========="
ls -l ${outdir}/${outname}
md5sum ${outdir}/${outname}
