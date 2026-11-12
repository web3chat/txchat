#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# 独立仓与 txchat/dtalk 并列；纳入后本目录是 dtalk/service/square。
PARENT="$(cd "$ROOT/.." && pwd)"
GRAND="$(cd "$ROOT/../.." && pwd)"
if [[ "$(basename "$GRAND")" == "dtalk" && -d "$GRAND/pkg/auth" ]]; then
  DTALK="$GRAND"
elif [[ -d "$PARENT/dtalk/pkg/auth" ]]; then
  DTALK="$PARENT/dtalk"
else
  DTALK=""
fi

if [[ -z "$DTALK" || ! -d "$DTALK/pkg/auth" ]]; then
  echo "skip check-auth-sync: dtalk pkg/auth not found"
  exit 0
fi

normalize() {
  tr -d '\r' | sed -e 's|github.com/web3chat/txchat/dtalk/pkg|PKG|g' -e 's|github.com/web3chat/txchat/dtalk/service/square/pkg|PKG|g'
}

fail=0
compare() {
  local rel="$1"
  local a="$DTALK/$rel"
  local b="$ROOT/$rel"
  if [[ ! -f "$a" ]]; then
    echo "missing in dtalk: $rel"
    fail=1
    return
  fi
  if [[ ! -f "$b" ]]; then
    echo "missing in square: $rel"
    fail=1
    return
  fi
  if ! diff -u <(normalize < "$a") <(normalize < "$b") >/dev/null; then
    echo "auth copy drifted: $rel"
    diff -u <(normalize < "$a") <(normalize < "$b") || true
    fail=1
  fi
}

files=(
  pkg/auth/apiauthenticator.go
  pkg/auth/apiauthenticator_test.go
  pkg/auth/apirequest.go
  pkg/auth/signatory.go
  pkg/auth/signatory_test.go
  pkg/address/address.go
  pkg/address/address_test.go
  pkg/address/error.go
  pkg/address/base58/alphabet.go
  pkg/address/base58/base58.go
  pkg/address/crypto/hash.go
  pkg/crypt/encrypt.go
  pkg/crypt/err.go
  pkg/crypt/secp256k1-haltingstate/plugin.go
  pkg/crypt/secp256k1-haltingstate/secp256k1.go
  pkg/crypt/secp256k1-haltingstate/secp256K1_test.go
  pkg/crypt/secp256k1-ethereum/plugin.go
  pkg/crypt/secp256k1-ethereum/secp256k1.go
  pkg/crypt/secp256k1-ethereum/secp256K1_test.go
)

for f in "${files[@]}"; do
  compare "$f"
done

if [[ "$fail" -ne 0 ]]; then
  echo "check-auth-sync failed: copy from dtalk again, only rewrite import prefix"
  exit 1
fi
echo "check-auth-sync ok"
