# QA delivery rebase — 2026-10-05

The user requested rebasing the completed QA slice onto main before opening its PR.
All 82 commits were replayed in order onto 2bc4e324d (PR #690), without squashing or
dropping work. The backup ref preserves pre-rebase HEAD 71260b142434eb11b40d40b5177bb3dd2760e9c0.

The impact-audit append conflict retains both sections. The migration reopen suite
retains both data-preservation cases. Main's 00126 reconciliation-index migration and
its checksum entry remain byte-identical; make codegen generated the unpublished
Profile provenance migration as 00127 and refreshed the checksum. Only the provenance
case now selects the prefix before 00127. No published migration or stored row is changed.

Historical receipts, bug fix SHAs and report checkpoints identify the pre-rebase series.
Use this map to locate their current equivalents. Post-rebase migration checks, codegen
verification and the required gate are recorded in the owning report and local receipts.

| Original commit | Rebased commit | Change |
|---|---|---|
| 196cd301003d839284c4cd0ec22f6ae40ae52fe4 | 35c0b829e3578024d2322f9789ae03734c5e0d4b | fix: preserve native tool discovery schemas |
| 42db6579ff822019017b7c394eec0a89a75da6f1 | 18b9f86b2e5ad589a6e68eaed33d59bfe54ca530 | fix: retain profile selection provenance |
| 951105e0fae1df51f52a48d19487b607bc65d53c | 362befb3867b88edb5e050d6676ff1ab22a67edc | fix: keep long project paths inside setup panels |
| 1c8e1043a79c0ea91ea7ffcdcfdec8c9c18dfe87 | b0f2ec76f5d9fb5b3bf2788fda1ea08f82bdd3ec | docs: record initial untested QA walks and repairs |
| 46d8b2f07a81b0c741b3b86511eaac78187c903a | 804a6b645d06a391e876bb53cdefd284c5598085 | fix: reject explicit zero overview windows |
| 8d1e73ab37e95a042f22e1a132e588d9fbe214a1 | 8645d0a4554e4efd6764154e0f76930b4ac4b6e6 | fix: preserve profile selection for task execution |
| 8df0742f90237a0a4be36c0c232c6550f6d92366 | 54e0accbdcd12b5944c43c5b186ae4375d3c780f | fix: preserve static resource manifest paths |
| 514de24d6a12019886627d14434d8e877d15bfa7 | b84670cf55fb8e288bacd15f8ce5f6cf5ff39245 | fix: keep multi-tab session controls responsive |
| 9f876f4f1ff162c5c0f8d1691615f512596e03f4 | e104e055f30ae2f58509e2529b95774e67aa0b5a | docs: record transport and extension QA replays |
| 212d9aea17d2f968350d31f27611b28fa64384ff | 43edea95528504dc3a7d173ef93e8213308b54f6 | fix: isolate refused histories during daemon startup |
| ebfb89518cf38c8d1682ae9dc8b142aaab933584 | c13a97808b82537f9eb93e1e51a4c7905d265cec | fix: preserve directory navigation after read failures |
| a5a70342a360944bb97ebac81c04c9c0d97f678e | cdd80f6bc26299cadc80b1166cccf479013d956f | fix: honor global scope when skipping project setup |
| 783304f77247362aeec01ec48969072916f543dd | 5a84e5ed264e36f1d2260d62a247d354a5a778da | fix: report the effective model in agent context |
| bef9a13b87521956c49466cada8587c39c91c7d4 | 5252449a653f3910edf256d4a7f94dd108b00933 | fix: bind approvals to canonical native tool input |
| 65f740194c2f28693deed173f5b720cad7949586 | d04b0dfdffe592b6ceb01923f4b6ef97aea82743 | docs: correct native tool hook matcher guidance |
| 5639904409eb56f681c5877a9a97a5ccd22804a9 | 76cbc550d440f79403bf5315aaeb67028e83589d | fix: normalize native workspace references to registration identity |
| 244020cfd89f2d97446a2e539f0820d91b0670d7 | d7a8f5ababe21c032161e9ee34874169b427b370 | fix: preserve native task catalog validation errors |
| e127956a32ef8ca4188994b50c72864e887b0e1d | 7ac6a067672c3a072eb89ea2216b88bdd32403f5 | fix: execute native tool hooks at registry dispatch |
| 7a7780ae3d2df261422bc87837d52acdcc6d6005 | feee410ce69479f793c98a3e2cc4f8ce0265542f | fix: preserve authorized task catalog workspace targets |
| e9e4a46a66298ee00218fccabbd573ca30d871fc | 50cd7e0be218d58fe37f07d7b511eb78b517d598 | fix: apply selected profile to approval commands |
| 4760da89ffa40a924397c8e2bbc10c2cb3db95f2 | 5b8a5265fff5a5a6f80071b238f5a5d15add9235 | fix: retain profile archive lifecycle events |
| fb4b8a40a1a74fd0ad579db2e82c90dd83e3d49b | 2fc24c2e3df38a20b34a24f04471e904ee2658b6 | fix: keep profile recovery stream alive |
| 8380b94f2d2b1933ef2c8e30655331165e210cd8 | 55d68e214ab24018c9e8cad0b6c5b0ccee4565d4 | fix: keep profile validation tied to current input |
| b4ab86b390db37c4a38779f7f24867ef55ec9645 | 03748abfc2cbefe2fd079dba33951fd8fb7a28fc | fix: preserve emoji keyboard navigation in profile dialogs |
| a15b2ea62d578324e59a2c91558884e78d6b3583 | 36f5456d71b404631ea98af4e5df9d76f2d6a50d | fix: select profile repository rename offers by default |
| b4ed8ca1823c570ebc5a117a32c0dd1b31ffe3cd | 0cbaf5085a89505df4620572b3e52a36fac326ea | fix: pause resource automations when archiving profiles |
| 74744060bbe56354f4fb5af6369b3174bdab4be2 | 267072faecfb38b688e5860e5f9e9b06b7bacbaf | fix: protect profile ownership and lifecycle navigation |
| b915570a8efd7ab0b0bfa95f3d559c430214008f | fa610a648eb5144c96cbe18a89f89f34cee30be5 | fix: restore remembered profiles between projects |
| fe8a644b1e2137664df34d210f40dbeafc503e09 | 55586b6fbbdbf5ea925459e14b38c53fb5b87627 | fix: preserve profile lifecycle recovery audits |
| 664f24775b2975aebe267758ddbb868d5e762a98 | 0584a714ea845d7fbdacaefed973961b005664e3 | fix: explain unavailable profile recovery |
| f38ee2ec19e738f6f27836d7c19c9fc34f1d7d40 | 95b061c5b89c648b916a467bf004a09e433c376e | fix: bind automation cursors after profile resolution |
| 61181c15d562a44651b4a8a406d5c9e99231a179 | 8737cd1f6f4b297ce6bd67d8a91f91e8b917f655 | fix: open project automation details in Global |
| 62b58628b62a03048967a91543323d3cb427bb7b | 103ea2b1a19a4aee1634b31ac23886ce37e7926b | fix: preserve Global scope during background window cleanup |
| 4c447e45e8cda069bde67869e2a61760efbeed7e | a3a557a19817af81fd18aa17cd8c8fb7120fdc5b | fix: show persisted task intent in job details |
| 78133b4f0f2c477f7fbc48b9abe1678463c0c5d0 | 1d88915121ef60ed50ddb630f746bc64632b108d | fix: keep trigger authoring and recovery actionable |
| 3f53932aa35300c32e92bb6a84d469e1d89f367c | 8a83e5852d4ef26b0d80979996d0316068b4b0b2 | fix: preserve global scope and explain paused catalog loading |
| 32417c5993950dd3d5a08291df20c3699679c41e | 707f73fb9d486b2e3eab87ad4e26081f4ad8362b | docs: record automation recovery verification |
| a2917318c96359b507b0e0a0fc06ad410f7dd648 | 6a9b87e35d7db7fcf2bfcdd08ed3c20595b4863c | fix: repair automation preview recovery and local web cache |
| 74e28290743c45d187e641ecec9dd4ed8c67690a | bdddf5a25b483ea53ab4046288cdf2caf4d3f1eb | docs: record automation preview and cache recovery evidence |
| 7a9d15e2f70cacb185f8cee5f02510aced982d34 | 1b32e53caadab9356a96aaa4e3b131bca11cd529 | fix: preserve upgraded global session histories |
| 831436907b384a00a76ab4405fda2bf93dcb7792 | 671ce289d537cd96fe8215508554a3fb717e0280 | fix: track extension profile creation provenance |
| 9f1457296693c4c400b730880ca8506587ad7ce9 | 2820cf0c29552c595485659330526e44e636235f | fix: close canceled approvals and refresh restart requirements |
| e76dfc37cc8fefd6877c5d05cf7e399fc397ef3c | 88afc5720a949c131c62c2af26d8b0513e920e89 | docs: record approval and settings replay evidence |
| 073b1705bb5ffafe2a2f85d57cfa9f5ac1405b92 | dbb3410d7de74e3e036ff7860466f66654a1dc4e | fix: resume palette approvals under their original owner |
| d31bf0558bab9c7a9fe26faed6a154d450ce4dd4 | 8677af7413e1ee7678141925ac166f5063a51d0c | fix: preserve task state and profile controls |
| 0b9c79779967d09e286ec6849f27ce43460d483b | d1feab029b27a3226ba5b4fd6d9592e33719a0a9 | fix: retain profile selection for task run controls |
| 7d30fa3e2c8f4b2b86cddcfbe4761d3a94e10b75 | 90da34d1fc69bf6c38214e39aaa2a60f8c7c7da1 | fix: preserve profile ownership and admission errors |
| 34028edadce173efe40b8e4c4ff111379b63f05e | e917d2dbd386447b08123b033253e571a1e2e96e | fix: preserve delegated profile command handoffs |
| 259d7142caea4f4a21353e82bc810fe9b74f19cb | 27fc581090bda9aed5004bdf5fd169c080caa978 | fix: preserve live command client state |
| 9c998792d1966f4de5a61ba120044f7df9edfa8e | 16b8d9003da4c2905cc78d87528ddad07269bcec | test: stabilize command client fixture formatting |
| 8d630a7f0b08b8dd9956075d720162167efe6da2 | 9c60b69a720540702890af3ecadc936cc723f477 | fix: preserve task profiles and refresh scheduler views |
| 09b7089b1a91097b8b0ec8a03656732a84e3e1ec | b293eaff88606899dad0bed26e7e1f446f7d8e7b | docs: record verified scheduler and task profile replays |
| 84f02d6b2c8105e4ee128b14e1c3a447755b6563 | 9b75b522af4c82c3264cac268b253154674a00cb | fix: reconcile attention policy after workspace deletion |
| 2ceadf8476a2a5a10510942d9d22e0eb6d66fda4 | 68371d35faecad47a873f084b8f70af4dbaffd65 | docs: record attention permission and suppression evidence |
| b4166a6c2fbfe10aeede40dd5091d23850f2409b | a2238d672ce2301060deaf09d8b14e8f067c1e26 | fix: restore settings search keyboard focus |
| 159a72e22ae08eb57132b74f758223154d2e96d1 | 4b1cd01f8185d0f6805f21bfc4cdb6fe5c581ca1 | docs: record settings shortcut QA and current visual references |
| 6aec027349214211c68315607d9ad7b4c5b93276 | c9f1ecad4ea783925ec1dba7fbc355ed1c46978f | fix: preserve editor drafts and help guidance |
| 0ba7a37d04f5f1bcb332744bb7628a918c27a01b | 2c31d431c9c6945ebb9e02d34a2368c08cd70cf3 | docs: record verified help guidance repairs |
| baec8d019cd6113ce7db0c2811118724eebd94a7 | 5718c9ac58f87af0e1e7302421603d7a2f4ff537 | fix: clarify vault guidance and compact settings |
| 9108465d990e09a339b6b387e86debb0a95d78ed | 79158b2b7c9ac4926ea82b11bcbd98cfec9bef88 | docs: record vault and modal verification |
| 3268b74772d374e193871761d158b7c9bb3fb9b7 | 221d2edb7d0291657315bc2a33d04f037d54a578 | fix: preserve saved idle timeout in settings |
| 4ce6fd811355affdd6a89bb1782b8bf2d3b4aaba | 0c3a95923ddfaa4e38d3b02a1971f18d8562e167 | fix: wait for settings command availability |
| 23dddb441a73fef816723f9feb3bfb2d2e3ceaeb | 106b56effd17ff844eb17a6b421b66619ee74970 | fix: recover settings saves after connection loss |
| 5002fcb3853884145ef996c953705c0ea05e9086 | 6eb39f0c298b5b6e20a0e403d7a9ef5fa701988f | docs: record settings recovery and concurrent attention QA |
| 52ed8d1bc92939c92793940c3302028b0d24064e | 10ba543fb4789e35a502cbad505f18fe86bd8941 | fix: resolve relative workspace registration paths |
| 60ddd98e12e10731a7f98d8dedca20ff88f2e459 | 20b27735bff9ab025d17ef5d9e57101c30f88ca9 | fix: preserve global session reads and complete loop catalog controls |
| 620d210994e83bea109bdfe18d41460fc096a3f2 | 92b252c5216b9f314e9e11654223b27227f3fc61 | docs: record four verified global and loop QA scenarios |
| 547027459508f5e2d550dcd80d5378e6ea077db3 | 2056709fd0f35d3755985b83658c8cb20c476068 | fix: enforce managed Loop tool restrictions |
| 30ef53d1bfe62e85732a05e9161c52564788b338 | f54aec439ed2f94513c2fcaee6647abd5593969d | docs: close Loop policy QA and record terminal findings |
| 5216cbba009d3ae6fb693e3cf801f2ae5484c66f | dd9ffbd905e66a55c9d84cfa04bb20dce496d216 | fix: stop canceled Loop prompts before output repair |
| 0d3161ab06f17d89bfc50f5e07ecec49afe7f5c1 | efa5479edc3482d8ef81c868f790bdd422dccda8 | docs: close cancellation replay and plan budget boundary QA |
| be93b40ea67fae3bc325a410160c87005f507eb3 | 016ad440c7a24a2ef6e8c285cdd148a8e72e0822 | fix: preserve truthful Loop stop and failure attribution |
| 20cbc5693d5e44d8157f994b9a71b11cf65b08f2 | 3535be3a08d961e6e38036ee822d284867b1cde6 | fix: honor initial gate routes and repeated blocker limits |
| acbeed2ec31a6d7c2f97fc82d00e0271d904ad70 | 237f3de7627c008ae3c105e7fd9efa8385f6f423 | fix: preserve loop review decisions and runs navigation |
| ed0648da9e075393bfc0fc2f4603219f26bd843d | 7a87a3d1bc540cc5c47f75e2cada8c3a356942fc | docs: close human review and runs QA findings |
| 1f497b4fe751696d687960c84b0f8cb5defa2be0 | 2c671beb00e48a4bd1cae915cc2007aedd73571e | fix: preserve profile scope in loop catalogs |
| e4c54e164cb0ea414bb5b074ece9e0100f00a4cb | 994e4d47d3b6cf59996eb67ac5f06be123831e04 | docs: close profile scoped loop catalog QA |
| 4dc707c76eb93e9587cbc6e3fedea3e66236377e | 0ba885105829a74481fd14ef991b6e4a16e28914 | fix: preserve loop task results |
| dcd2c7ac84c51eedf7ba4f87aff7f189e7f3add6 | 81ae978731e5bb7ede146e93ed2ab64261f37cfc | docs: record verified task delivery walks |
| 24c704df17ed3237540063fc546f4fad9c080e5a | 3001e9da65bf41f16d67b4f501c643058917933d | docs: correct durable wait event example |
| 4bb63bebe8029e98e481532b1fae8b25f0944814 | c20d1897d617871c0842bca7a913de03088f3278 | docs: record pinned wait recovery |
| 71260b142434eb11b40d40b5177bb3dd2760e9c0 | 991f7f0681afc09d44632cc12d59cf230c34e1dd | docs: close QA scope and defer remaining scenarios |
