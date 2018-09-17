#!/bin/bash


OUT="./bin/"

VERSION=`date -u +%Y%m%d%H%M`
LDFLAGS="-X main.VERSION=$VERSION -s -w"
GCFLAGS=""

ARCHS=(x64 arm8 arm7 x86 win64 win32 mipsle)

startgo() {
NAME=$1
shift
case "$NAME" in
'x64')
	rungo linux amd64 7 "$@"
;;
'x86')
	rungo linux 386 7 "$@"
;;
'arm5')
	rungo linux arm 5 "$@"
;;
'arm7')
	rungo linux arm 7 "$@"
;;
'arm8')
	rungo linux arm64 7 "$@"
;;
'win32')
	rungo windows 386 7 "$@"
;;
'win64')
	rungo windows amd64 7 "$@"
;;
'mac64')
	rungo darwin amd64 7 "$@"
;;
'mipsle')
	rungo linux mipsle 7 "$@"
;;
'mips')
	rungo linux mipsle 7 "$@"
;;

esac
}

rungo() {
	OS=$1
	shift
	ARCH=$1
	shift
	ARM=$1
	shift
	echo "[$OS, $ARCH, $ARM]": "$@"
#	docker run --rm -v $PWD:/usr/src/myapp -w /usr/src/myapp -u $(id -u):$(id -g) -e GOOS=$OS -e GOARCH=$ARCH -e GOARM=$ARM $VERSION go "$@"
	GOOS=$OS GOARCH=$ARCH GOARM=$ARM go "$@"
}


for v in ${ARCHS[@]}; do
	startgo $v build -o $OUT/widdly-$v.elf -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" .
	#go-$v build -o $OUT/widdly-$v.elf -ldflags "$LDFLAGS" -gcflags "$GCFLAGS" .
done

mv $OUT/widdly-win32.elf $OUT/widdly-win32.exe
mv $OUT/widdly-win64.elf $OUT/widdly-win64.exe
cp index.html $OUT

cd $OUT
for v in *.elf; do
	#zip ${v%.elf}.zip $v index.html
	tar -czf ${v%.elf}.tar.gz $v index.html
done

for v in *.exe; do
	zip ${v%.exe}.zip $v index.html
	#tar -czf ${v%.exe}.tar.gz $v index.html
done

sha256sum *.tar.gz
sha256sum *.zip

