package main

// Flutter install guidance printed by doctor when Flutter is missing.
//
// devtools does not install Flutter itself: putting it on PATH and installing
// the native desktop toolchain differ per OS and need admin rights or a package
// manager, so doctor prints the exact steps for this OS instead (approved; see
// docs/adr/0003-dependency-policy.md, "Dev tool setup").

const (
	flutterRepo = "https://github.com/flutter/flutter.git"
	darwin      = "darwin"
)

// flutterInstallSteps returns numbered steps to install Flutter stable and the
// desktop toolchain for goos. Paths use the shell's own home variable so the
// lines can be pasted as they are.
func flutterInstallSteps(goos string) []string {
	dir, pathStep, toolchain := "$HOME/develop/flutter", "", ""
	switch goos {
	case windows:
		dir = `$env:USERPROFILE\develop\flutter`
		pathStep = `add it to PATH (PowerShell, then open a new terminal): ` +
			`[Environment]::SetEnvironmentVariable('Path', ` +
			`[Environment]::GetEnvironmentVariable('Path', 'User') + ';' + "` + dir + `\bin", 'User')`
		toolchain = `install Visual Studio 2022 with the "Desktop development with C++" workload: ` +
			`https://visualstudio.microsoft.com/downloads/`
	case darwin:
		pathStep = `add it to PATH: echo 'export PATH="` + dir + `/bin:$PATH"' >> ~/.zprofile && source ~/.zprofile`
		toolchain = "install Xcode from the App Store, then: sudo xcodebuild -runFirstLaunch && brew install cocoapods"
	default:
		pathStep = `add it to PATH: echo 'export PATH="` + dir + `/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc`
		toolchain = "install the desktop toolchain (Debian/Ubuntu; same packages elsewhere): " +
			"sudo apt install clang cmake ninja-build pkg-config libgtk-3-dev"
	}
	return []string{
		"Flutter is needed only for the desktop app. To install Flutter stable:",
		"1. get the SDK: git clone -b stable " + flutterRepo + " " + dir,
		"2. " + pathStep,
		"3. " + toolchain,
		"4. check: flutter doctor   (fixes for anything it marks [!] or [x])",
		"5. then: task install      (fetches the desktop app's packages)",
		"guide: https://docs.flutter.dev/get-started/install",
	}
}
