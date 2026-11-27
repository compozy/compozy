package remotehost

const cloudflaredVersion = "2026.11.2"

type cloudflaredPin struct {
	asset, assetSHA256, binarySHA256 string
}

// Digests come from the upstream release assets. The macOS archives also pin the extracted executable.
var cloudflaredPins = map[string]cloudflaredPin{
	"linux/amd64": {
		asset:       "cloudflared-linux-amd64",
		assetSHA256: "56f13335143ec0e22772963290683a84a04114a35ec64cbc794a4d39ff9bb8cf",
	},
	"linux/arm64": {
		asset:       "cloudflared-linux-arm64",
		assetSHA256: "04a7b7f08bce85d56b26ffe1fdbfbb21a074495a363d16f0783c40e25b5c5281",
	},
	"darwin/amd64": {
		asset:        "cloudflared-darwin-amd64.tgz",
		assetSHA256:  "6153d9ec629174882a3adbff2f7dc5a24b4f3bdd7ccc38eaa84fba26eb890311",
		binarySHA256: "870dc2094f98befdd993c455234562be4eee51f8fca6e2e356df4579fa7e6766",
	},
	"darwin/arm64": {
		asset:        "cloudflared-darwin-arm64.tgz",
		assetSHA256:  "f6d58eeec91ad1ec5fd1f7e4a587303772929b2625be5d24068033851c36ff55",
		binarySHA256: "8654fbdc9057f3e13e95f7892f417f8643113bee1b91ee2388fd0acb05210d55",
	},
	"windows/amd64": {
		asset:       "cloudflared-windows-amd64.exe",
		assetSHA256: "555b46c85582461d189cac30d14ad05480d612613801be9f110b8ce08c5b2113",
	},
}
