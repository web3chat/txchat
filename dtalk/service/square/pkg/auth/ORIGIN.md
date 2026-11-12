# Auth copy origin

- source: dtalk @ 8afecc8aac2942a6c605cdfe1d2efad576b7aaab
- rule: only rewrite import prefix `github.com/web3chat/txchat/dtalk/pkg` → `github.com/web3chat/txchat/dtalk/service/square/pkg`; do not change algorithm
- extra in square (not compared): `pkg/auth/golden_test.go`, `pkg/auth/testdata/`

Copied packages: `pkg/auth`, `pkg/address`, `pkg/crypt` (encrypt + secp256k1-haltingstate + secp256k1-ethereum).

## sha256

| file | sha256 |
|------|--------|
| `pkg/auth/apiauthenticator.go` | `488f79920770312b5ca27986acdf332c71fb147e731039797d9fdfba3f80f3b0` |
| `pkg/auth/apiauthenticator_test.go` | `27a6100514172654e938d0a4714f502305f7bf20dc0ba91f3f344f296e8ee1e4` |
| `pkg/auth/apirequest.go` | `db9a14605430d958dac47a61e863720e49094094cf51eaee14dea3d0f805e261` |
| `pkg/auth/signatory.go` | `34711f3095edf4f4fbb51fce984c2706ea2bbfa9b59e9f25a4c79dd2dfa5191b` |
| `pkg/auth/signatory_test.go` | `811c44ad5e9c0bfa322aa7b73a878d95752350b0c03dffce286bbc0d55ea6620` |
| `pkg/address/address.go` | `b256b12d45397dab63f5af8b62889afc01a99cd4e66f12ac49bf362f0b28b7e3` |
| `pkg/address/address_test.go` | `4c3ba7eb90bbb4818d66a4d5e7274bf97f0c2070e8a7e93053ef455063cff7b2` |
| `pkg/address/base58/alphabet.go` | `0c7e9684c85cff122e73a6664d3a5a57017084a54b989a02249c062d4be711bc` |
| `pkg/address/base58/base58.go` | `ee2898c5fb16d0625950bb660959be899c8dd8e6940f35a3c4a28fd9e4b37c87` |
| `pkg/address/crypto/hash.go` | `9cb8d64b877cc95271c33557578341d831f2314f4e294759fc98492544ba9158` |
| `pkg/address/error.go` | `8c4833f57f119f21c308f5b4b4e790150b3087a885b59cc49ee6ab05fb65ea57` |
| `pkg/crypt/encrypt.go` | `5c03ee3c3dcb5772c6bdb8fb73d353ef7c2e0b7500075265d20a0f6547834048` |
| `pkg/crypt/err.go` | `22914b5b80f99e9620d84db8227efd5ab960fce28cd8cd2398801540293e8e9c` |
| `pkg/crypt/secp256k1-ethereum/plugin.go` | `d9375cd12fa2860918fcf0cbb563e7413bb3836b8470d0465c9f3c8797ffac3b` |
| `pkg/crypt/secp256k1-ethereum/secp256K1_test.go` | `01ab8eebb971ff3f1bd2be01f3c894721e31b30da98e2033a5a3c835787c5a81` |
| `pkg/crypt/secp256k1-ethereum/secp256k1.go` | `0dd0eea8bef6b50cf7d41a8e218b3d64facfacabc0b34d30fea47ea43000087f` |
| `pkg/crypt/secp256k1-haltingstate/plugin.go` | `cfa1e56af2e238ba3b3e12594870f9e203471f1464a66e5dbdddc88d1d6832fc` |
| `pkg/crypt/secp256k1-haltingstate/secp256K1_test.go` | `9b21b519375bfdb6832f2ccd8169c9cc8ca55c9c8685db8ecf2a9a8afca92b24` |
| `pkg/crypt/secp256k1-haltingstate/secp256k1.go` | `e74525b2972f11d2bda8d3ba4ce7478f745bfa494342e6840e3f815e5a43bba3` |
