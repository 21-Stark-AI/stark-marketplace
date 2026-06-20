package provenance

import (
	"crypto/sha256"
	"encoding/hex"
)

// signerIdentity is the EXACT keyless signer identity that may produce a valid build
// manifest: the sign-manifest workflow running on the main ref. It is matched exactly
// (not by a prefix regexp) so that NO other workflow in this repo — e.g. web-deploy.yml,
// which also holds id-token: write — and no non-main ref can mint an accepted signature.
// The OIDC issuer additionally pins GitHub Actions as the token source.
const (
	signerIdentity = "https://github.com/21-Stark-AI/stark-marketplace/.github/workflows/sign-manifest.yml@refs/heads/main"
	oidcIssuer     = "https://token.actions.githubusercontent.com"
)

// VerifyDigests recomputes sha256 over the provided files and returns the paths whose digest
// does NOT match the manifest (or are missing). Empty slice = match. This is the ANTI-DRIFT
// layer; the cosign signature (CosignVerifyCmd) is provenance.
func VerifyDigests(m *BuildManifest, files map[string][]byte) []string {
	var mismatches []string
	for _, fd := range m.Files {
		data, ok := files[fd.Path]
		if !ok {
			mismatches = append(mismatches, fd.Path)
			continue
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != fd.Digest {
			mismatches = append(mismatches, fd.Path)
		}
	}
	return mismatches
}

// CosignVerifyCmd returns the argv for keyless verification of a signed manifest. stark
// verify-manifest shells out to this (cosign must be on PATH). The signer identity (exact)
// + OIDC issuer are pinned so a signature from any other workflow, ref, or identity fails.
func CosignVerifyCmd(manifest, sig, cert string) []string {
	return []string{
		"cosign", "verify-blob",
		"--certificate-identity", signerIdentity,
		"--certificate-oidc-issuer", oidcIssuer,
		"--signature", sig,
		"--certificate", cert,
		manifest,
	}
}
