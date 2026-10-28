#!/usr/bin/env bash
#
# build.sh builds the small, LGPL-licensed ffmpeg that is shipped inside the
# my-capture downloads, so users do not have to install ffmpeg themselves.
#
# Usage: build.sh macos|windows OUTPUT_DIR
#
# It downloads the FFmpeg source release named below, checks its checksum,
# builds an ffmpeg that contains only what my-capture uses, and writes into
# OUTPUT_DIR:
#
#   ffmpeg (or ffmpeg.exe)   the program; a universal arm64 + x86_64 binary on macOS
#   COPYING.LGPLv2.1         the licence text, from the FFmpeg source
#   FFMPEG-NOTICE.txt        where to get the source, and how this copy was built
#
# WHY BUILD IT?
# Ready-made macOS ffmpeg builds are almost all GPL licensed and contain far
# more than is needed, and Homebrew's copy depends on dozens of shared
# libraries elsewhere on the disk. A build with only the needed parts is about
# 5 MB instead of 50 to 100 MB, has no outside dependencies, and is under the
# LGPL, whose terms are easy to meet when redistributing: ship the licence and
# say where the source is (FFMPEG-NOTICE.txt does both). Nothing GPL or
# non-free is enabled, and no external libraries are used.
#
# WHAT my-capture USES FROM FFMPEG
#   input    the avfoundation (macOS) or dshow (Windows) capture device
#   decode   rawvideo (what capture cards deliver), mjpeg and h264 (Windows
#            cards that send compressed video)
#   convert  the scale and format filters (swscale), to change whatever the
#            device delivers into 8 bit RGBA
#   output   the rawvideo encoder and muxer, written to a pipe
# If my-capture ever needs more (another device type, another codec), add it
# to COMPONENTS below.
#
# Environment variables (all optional):
#   FFMPEG_JOBS   number of parallel compile jobs. Default: all processors.
#   MACOS_MIN     oldest macOS version the build runs on. Default: 12.0.

set -euo pipefail

# -----------------------------------------------------------------------
# Version being built
# -----------------------------------------------------------------------

# FFMPEG_VERSION is the FFmpeg release to build. To upgrade, change it, take
# the checksum of the new tarball from a download you trust, and check that
# "make dist" still produces a working ffmpeg.
FFMPEG_VERSION="9.0.2"
FFMPEG_SHA256="8c3850283eb25fa026482078a04051e0be17347b09ef81a0849bec15a96e002e"
FFMPEG_URL="https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"

# -----------------------------------------------------------------------
# Arguments
# -----------------------------------------------------------------------

if [ $# -ne 2 ]; then
	echo "usage: $0 macos|windows OUTPUT_DIR" >&2
	exit 2
fi
platform="$1"
out="$(mkdir -p "$2" && cd "$2" && pwd)"
macos_min="${MACOS_MIN:-12.0}"

case "$platform" in
macos | windows) ;;
*)
	echo "unknown platform '$platform': use macos or windows" >&2
	exit 2
	;;
esac

# jobs is the number of parallel compile jobs.
jobs="${FFMPEG_JOBS:-$( (nproc || sysctl -n hw.ncpu || echo 2) 2>/dev/null)}"

# sha256 prints the SHA-256 checksum of a file, whichever tool is installed.
sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# -----------------------------------------------------------------------
# What to build
# -----------------------------------------------------------------------

# COMPONENTS is everything switched on in the otherwise empty build.
COMPONENTS=(
	--enable-ffmpeg
	--enable-swscale
	--enable-muxer=rawvideo
	--enable-encoder=rawvideo
	--enable-decoder=rawvideo,mjpeg,h264
	--enable-parser=mjpeg,h264
	--enable-protocol=pipe,file
	--enable-filter=scale,format,null,buffer,buffersink
)

# COMMON is the rest of the configure line, the same on every platform.
COMMON=(
	--disable-everything
	--disable-autodetect # no optional external libraries
	--disable-doc
	--disable-debug
	--disable-ffplay
	--disable-ffprobe
	--disable-network
)

# -----------------------------------------------------------------------
# Source
# -----------------------------------------------------------------------

src_dir="$out/src"
mkdir -p "$src_dir"
tarball="$src_dir/ffmpeg-${FFMPEG_VERSION}.tar.xz"
source_tree="$src_dir/ffmpeg-${FFMPEG_VERSION}"

if [ ! -f "$tarball" ]; then
	echo "Downloading $FFMPEG_URL"
	curl -fsSL -o "$tarball" "$FFMPEG_URL"
fi
if [ "$(sha256 "$tarball")" != "$FFMPEG_SHA256" ]; then
	echo "error: $tarball does not match the expected SHA-256 checksum" >&2
	rm -f "$tarball"
	exit 1
fi
if [ ! -d "$source_tree" ]; then
	tar -xf "$tarball" -C "$src_dir"
fi

# Note for editors: macOS ships bash 3.2, where expanding an empty array under
# "set -u" is an error, hence the ${array[@]+"${array[@]}"} form below.

# build_one configures and compiles one copy of ffmpeg in its own directory
# and leaves the result at $out/ffmpeg-NAME. The remaining arguments are extra
# configure options for this copy.
build_one() {
	local name="$1"
	shift
	local dir="$out/build-$name"
	rm -rf "$dir"
	mkdir -p "$dir"
	(
		cd "$dir"
		"$source_tree/configure" "${COMMON[@]}" "${COMPONENTS[@]}" "$@"
		# The make target is "ffmpeg.exe" on Windows and "ffmpeg" elsewhere, and
		# configure records which in EXESUF.
		exesuf="$(sed -n 's/^EXESUF=//p' ffbuild/config.mak)"
		make -j"$jobs" "ffmpeg$exesuf"
	)
	# The program is called ffmpeg.exe on Windows. The copy keeps the suffix.
	local exe="ffmpeg"
	[ -f "$dir/ffmpeg.exe" ] && exe="ffmpeg.exe"
	cp "$dir/$exe" "$out/ffmpeg-$name${exe#ffmpeg}"
	rm -rf "$dir" # the build directory is hundreds of megabytes
}

# x86_asm lists the configure option that switches the x86 assembly off when
# the nasm assembler is not installed. Without assembly the colour conversion
# is several times slower, so release builds must have nasm.
x86_asm() {
	if ! command -v nasm >/dev/null 2>&1; then
		echo "warning: nasm not found, building without x86 assembly (slower)" >&2
		echo "--disable-x86asm"
	fi
}

# -----------------------------------------------------------------------
# Platform builds
# -----------------------------------------------------------------------

case "$platform" in
macos)
	# Build for both processor types and join them into one universal binary.
	# The copy for the processor this is not running on is cross-compiled, so
	# no test program has to run on it (Rosetta may not be installed).
	host="$(uname -m)"
	[ "$host" = "arm64" ] && host_arch="arm64" || host_arch="x86_64"

	for arch in arm64 x86_64; do
		cross=()
		[ "$arch" != "$host_arch" ] && cross=(--enable-cross-compile)
		asm=()
		[ "$arch" = "x86_64" ] && asm=($(x86_asm))
		build_one "$arch" \
			--target-os=darwin --arch="$arch" ${cross[@]+"${cross[@]}"} ${asm[@]+"${asm[@]}"} \
			--cc="clang -arch $arch" \
			--extra-cflags="-mmacosx-version-min=$macos_min" \
			--extra-ldflags="-mmacosx-version-min=$macos_min" \
			--enable-avfoundation --enable-indev=avfoundation
	done
	lipo -create -output "$out/ffmpeg" "$out/ffmpeg-arm64" "$out/ffmpeg-x86_64"
	strip -x "$out/ffmpeg"
	chmod +x "$out/ffmpeg"
	rm -f "$out/ffmpeg-arm64" "$out/ffmpeg-x86_64"
	;;
windows)
	# Normally run in an MSYS2 MINGW64 shell with gcc, make, nasm and diffutils
	# installed. Run anywhere else (macOS or Linux with MinGW-w64), it
	# cross-compiles instead.
	# -static puts the compiler's runtime into the program, so it needs no DLLs
	# from the MinGW installation.
	cross=()
	prefix=""
	case "$(uname -s)" in
	MINGW* | MSYS*) ;;
	*)
		prefix="x86_64-w64-mingw32-"
		cross=(--enable-cross-compile "--cross-prefix=$prefix")
		;;
	esac
	build_one x86_64 \
		--target-os=mingw32 --arch=x86_64 ${cross[@]+"${cross[@]}"} $(x86_asm) \
		--extra-ldflags="-static" \
		--enable-indev=dshow
	mv "$out/ffmpeg-x86_64.exe" "$out/ffmpeg.exe"
	"${prefix}strip" "$out/ffmpeg.exe"

	# The program must not need any DLL that is not part of Windows itself.
	echo "DLLs used by ffmpeg.exe:"
	"${prefix}objdump" -p "$out/ffmpeg.exe" | grep "DLL Name" | sed 's/^[[:space:]]*/  /'
	if "${prefix}objdump" -p "$out/ffmpeg.exe" | grep -qiE "DLL Name: (libwinpthread|libgcc|libstdc)"; then
		echo "error: ffmpeg.exe depends on MinGW runtime DLLs" >&2
		exit 1
	fi
	;;
esac

# -----------------------------------------------------------------------
# Licence and notice
# -----------------------------------------------------------------------

cp "$source_tree/COPYING.LGPLv2.1" "$out/COPYING.LGPLv2.1"
cat >"$out/FFMPEG-NOTICE.txt" <<EOF
FFmpeg
======

This package contains a copy of ffmpeg ${FFMPEG_VERSION}, which my-capture runs
to read video from the capture device. FFmpeg is free software, licensed
under the GNU Lesser General Public License, version 2.1 or later. The licence
text is in COPYING.LGPLv2.1, next to this file.

This copy was built from the unmodified FFmpeg source release:

    ${FFMPEG_URL}
    SHA-256 ${FFMPEG_SHA256}

It contains only the parts my-capture needs, and no GPL or non-free code and
no external libraries. The build script, with the exact configuration, is
packaging/ffmpeg/build.sh in the my-capture source:

    https://github.com/steveohara/my-capture

You may replace this ffmpeg with another build: my-capture uses the ffmpeg
next to it if there is one, otherwise the one on the PATH, or the one named
by the MYCAPTURE_FFMPEG environment variable.

FFmpeg is a trademark of Fabrice Bellard, originator of the FFmpeg project.
See https://ffmpeg.org/legal.html
EOF

# Keep only the results: the extracted source tree is large, and the tarball
# alone is enough to build again.
rm -rf "$source_tree"

echo "Built ffmpeg ${FFMPEG_VERSION} for ${platform} in ${out}"
