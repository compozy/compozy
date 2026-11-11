package loop

// Remove with the pre-view shim in the first release after the one shipping views (issue #1462).
var supersededActionOutputDigests = map[string]struct{ old, corrected string }{
	"compozy__cmd_palette_invoke": {
		"75edc382c23b8a0de4c904f331610122712673f166745c970f0a54e1e0cf89dc",
		"3b2dec049ae423238e543aa7479c8236001ab02837bc8293d88a871229a4b885",
	},
	"compozy__loop_nodes": {
		"067d408f91b303e2ff0b549b77cbab22a40a5054cddc41df904913836a52d505",
		"1c4e59d28430d2121299eaad3644b04528d70221a5e73254727c8e52c60ff198",
	},
	"compozy__loop_request": {
		"6937b9c26ee30f39cedbb8b750c24c146e5583f19e03ecf1b2a09aaa0c927b06",
		"a538a3d1c52b8623af87173c374e7124bf70c2b252a226c956c303f8c4588b62",
	},
	"compozy__loop_requests": {
		"57837814e377a35c0766453eaa917c26c7915f2de1683e89ce8d95db4b1df88b",
		"68a33fd245bc7412f25067ebf635dc53f96521fc1d85696e4420dbf987d01e3a",
	},
	"compozy__loop_runs": {
		"9a2e5457070fc9788d755717c7bb28dc6082fe389c47f4f8a4a18c3e18ca5d87",
		"782e15aa3e1a5325c6633092b765e0c97ea21f00a9fab58e395f54a35eff6b20",
	},
	"compozy__loop_status": {
		"280b5ca2dee26b872c12a951e1c6f43bafabad5bb4df35031f1ff9b0dc532a3d",
		"8b3eecb0e391806775601855fec631d41b8402fadb19e4c70044a5df1f157d85",
	},
	"compozy__tool_approvals_list": {
		"941067bd81c6553ee7b51dab270340f6de777377122c6a59b4803ca7189ab915",
		"41a849e9ed2f378c4a126b0260a76a4a24d02b6db0ffaba91f51535e9b077b9d",
	},
	"compozy__tool_approvals_set": {
		"cfe185c0209cbb2e25cad7cbaa1d66e5915cc6e3ca9f436a89f6bcf22874c02e",
		"277280b4604f1d81f9f21788a66e610eccc2c2c220615c1f880da07200d2ab48",
	},
}
