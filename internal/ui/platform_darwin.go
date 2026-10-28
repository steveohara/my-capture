//go:build darwin

// Package ui: platform_darwin.go makes a program that was started without a
// macOS application bundle look like a proper application.
//
// THE PROBLEM
// A macOS application gets its name and icon from the Info.plist and icon
// inside its .app bundle (see assets/macos and "make dist-macos"). A program
// started straight from a terminal or an IDE has no bundle, so macOS names it
// after its executable file, which can be something like "___1App" when an IDE
// builds it to a temporary place. That name appears in the menu bar, the Dock
// tooltip and the application switcher, next to a generic icon.
//
// THE FIX, IN TWO PARTS
//  1. Relaunch. The Dock decides the label under an icon when it first hears of
//     a program, and no later change of name is picked up, so the only reliable
//     fix is to be in a bundle from the start. relaunchInBundle builds a small
//     "My Capture.app" in the user cache folder (Info.plist and icon from the
//     embedded assets, and a link to or copy of the running executable) and
//     replaces the process with the copy inside it. The process keeps its
//     process ID, so an IDE's Stop button still works. This is skipped when the
//     program is already in a bundle, when MYCAPTURE_NO_RELAUNCH is set, and
//     when a debugger is attached, because a debugger loses a process that
//     replaces itself.
//  2. Setting identity. When the relaunch is skipped, setProgramIdentity still
//     gives the menu bar the right name and the Dock the right icon. It cannot
//     fix the Dock tooltip, which keeps the executable's file name; that is the
//     price of debugging.
//
// Both run before Fyne starts, because GLFW reads the name when it creates the
// menu bar.
package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <crt_externs.h>
#include <stdlib.h>
#include <string.h>

// setProgramIdentity sets the name used for the application menu and, when
// the program is not inside an application bundle, the Dock icon.
static void setProgramIdentity(const char* name, const void* png, int pngLen) {
	// GLFW falls back to the program name when there is no bundle. The name
	// string is kept for the life of the process, as _NSGetProgname expects.
	NSString* title = [NSString stringWithUTF8String:name];
	char** progname = _NSGetProgname();
	if (progname != NULL) {
		*progname = strdup(name);
	}
	[[NSProcessInfo processInfo] setProcessName:title];

	// The bold name in the menu bar comes from the bundle information, which
	// for a program without a bundle is a synthesised dictionary holding the
	// executable's file name. It is a mutable dictionary in practice, so the
	// real name is written into it before the menu bar is built.
	NSDictionary* info = [[NSBundle mainBundle] infoDictionary];
	if ([info respondsToSelector:@selector(setObject:forKey:)]) {
		[(NSMutableDictionary*)info setObject:title forKey:@"CFBundleName"];
		[(NSMutableDictionary*)info setObject:title forKey:@"CFBundleDisplayName"];
	}

	if ([[NSBundle mainBundle] bundleIdentifier] != nil) {
		return; // a real bundle supplies its own icon
	}

	// NSApp does not exist yet, and creating it here would stop GLFW from
	// installing its own application class. Setting the icon is queued for the
	// main thread instead, and runs once the event loop has started.
	NSData* data = [NSData dataWithBytes:png length:pngLen];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSImage* image = [[NSImage alloc] initWithData:data];
		if (image != nil && NSApp != nil) {
			[NSApp setApplicationIconImage:image];
		}
	});
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"my-capture/assets"
)

// -----------------------------------------------------------------------
// Entry point
// -----------------------------------------------------------------------

// platformInit is called before the Fyne application is created. Unless
// noRelaunch is set it first tries to restart the program inside a temporary
// application bundle, and then (if that is not done) makes macOS show name in
// the menu bar and icon (PNG data) in the Dock.
func platformInit(name, version string, icon []byte, noRelaunch bool) {
	if !noRelaunch {
		relaunchInBundle(name, version)
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	if len(icon) == 0 {
		C.setProgramIdentity(cName, nil, 0)
		return
	}
	C.setProgramIdentity(cName, unsafe.Pointer(&icon[0]), C.int(len(icon)))
}

// -----------------------------------------------------------------------
// Relaunching inside a bundle
// -----------------------------------------------------------------------

const (
	// relaunchedEnv is set in the environment of the restarted program. It
	// stops a restart from being repeated if the bundle somehow does not take
	// effect, which would otherwise loop forever.
	relaunchedEnv = "MYCAPTURE_RELAUNCHED"

	// pTraced is the P_TRACED flag in a process's flags: a debugger is
	// attached.
	pTraced = 0x800
)

// relaunchInBundle replaces the running process with a copy of itself inside
// a temporary application bundle. It returns only if no restart was done or
// the restart failed; failures are reported on standard error and are not
// fatal, since the program works fine without the bundle.
func relaunchInBundle(name, version string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if os.Getenv(relaunchedEnv) != "" || strings.Contains(exe, ".app/Contents/MacOS/") || underDebugger() {
		return
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	app := filepath.Join(dir, "my-capture", "dev", name+".app")
	dst, err := buildBundle(app, exe, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not make an app bundle, continuing without: %v\n", err)
		return
	}

	// Exec keeps the process ID and replaces the program. It only returns on
	// failure.
	env := append(os.Environ(), relaunchedEnv+"=1")
	err = syscall.Exec(dst, append([]string{dst}, os.Args[1:]...), env)
	fmt.Fprintf(os.Stderr, "warning: could not restart inside the app bundle, continuing without: %v\n", err)
}

// buildBundle writes the application bundle at app, holding exe, and returns
// the path of the executable inside it.
func buildBundle(app, exe, version string) (string, error) {
	macos := filepath.Join(app, "Contents", "MacOS")
	resources := filepath.Join(app, "Contents", "Resources")
	for _, d := range []string{macos, resources} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", fmt.Errorf("create %s: %w", d, err)
		}
	}

	plist := bytes.ReplaceAll(assets.InfoPlist, []byte("@VERSION@"), []byte(version))
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), plist, 0o644); err != nil {
		return "", fmt.Errorf("write Info.plist: %w", err)
	}
	if err := os.WriteFile(filepath.Join(resources, "icon.icns"), assets.IconICNS, 0o644); err != nil {
		return "", fmt.Errorf("write icon: %w", err)
	}

	// The executable name must match CFBundleExecutable in the Info.plist.
	dst := filepath.Join(macos, "my-capture")
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace %s: %w", dst, err)
	}
	// A hard link costs nothing and cannot go stale, but only works on the
	// same disk; otherwise the executable is copied.
	if err := os.Link(exe, dst); err != nil {
		if err := copyFile(exe, dst); err != nil {
			return "", fmt.Errorf("copy executable: %w", err)
		}
	}
	return dst, nil
}

// copyFile copies the file src to dst and makes it executable.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// underDebugger reports whether a debugger is attached to this process.
func underDebugger() bool {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", os.Getpid())
	return err == nil && kp.Proc.P_flag&pTraced != 0
}
