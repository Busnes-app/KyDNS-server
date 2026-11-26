#!/usr/bin/env bash
# Fail if non-test Go code sets CurvePreferences directly, if code/workflows/
# Dockerfiles set tlsmlkem, or if the PQ TLS handshake tests fail in either
# default mode or GODEBUG=fips140=on.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Non-test Go code must not set CurvePreferences directly: Ky-to-Ky links pin
# hybrid key exchange via pqtls.HybridOnly, and outbound links rely on Go's
# default preference for X25519MLKEM768.
curve_hits=$(git grep -n -w 'CurvePreferences' -- '*.go' ':!*_test.go' \
  | grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' || true)
if [ -n "$curve_hits" ]; then
  echo "Non-test Go code sets CurvePreferences directly (use pqtls.HybridOnly or Go defaults):" >&2
  echo "$curve_hits" >&2
  exit 1
fi

# Code, workflows, Dockerfiles, Compose, Makefile, packaging, and scripts must
# not override ML-KEM TLS key exchange via GODEBUG=tlsmlkem=0.
tlsmlkem_hits=$(git grep -n -i 'tlsmlkem' -- \
  '*.go' \
  '.github/' \
  '*Dockerfile*' \
  'docker-compose*.yml' 'docker-compose*.yaml' \
  'Makefile' \
  'packaging/' \
  'scripts/' \
  ':!scripts/check-pqc-tls.sh' || true)
if [ -n "$tlsmlkem_hits" ]; then
  echo "tlsmlkem override is forbidden:" >&2
  echo "$tlsmlkem_hits" >&2
  exit 1
fi

run_pq_tests() {
  local label="$1"
  shift
  local out
  out=$("$@" go test -count=1 -v \
    -run '^(TestReplicaLinkNegotiatesHybrid|TestReplicaLinkRefusesX25519OnlyPeer|TestOutboundTLSNegotiatesHybrid)$' \
    ./internal/replica ./internal/upstream)
  echo "=== PQC TLS handshake tests ($label) ==="
  echo "$out"
  for name in TestReplicaLinkNegotiatesHybrid TestReplicaLinkRefusesX25519OnlyPeer TestOutboundTLSNegotiatesHybrid; do
    if ! grep -q -- "--- PASS: ${name}" <<<"$out"; then
      echo "Expected ${name} to pass under ${label}" >&2
      exit 1
    fi
  done
  if grep -q -- "--- SKIP" <<<"$out"; then
    echo "Unexpected skipped PQ TLS test under ${label}" >&2
    exit 1
  fi
}

run_pq_tests "default" env -u GODEBUG
run_pq_tests "GODEBUG=fips140=on" env GODEBUG=fips140=on
