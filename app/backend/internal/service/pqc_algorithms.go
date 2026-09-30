package service

import (
	"crypto/rand"
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
)

// pqcAlgorithm adapts one FIPS 204 ML-DSA parameter set. Signing is always
// hedged (randomized), matching how existing ML-DSA-65 proofs were produced.
type pqcAlgorithm struct {
	generate func() (publicKey, privateKey []byte, err error)
	sign     func(privateKey, message, context []byte) ([]byte, error)
	verify   func(publicKey, message, context, signature []byte) bool
}

var pqcAlgorithms = map[string]pqcAlgorithm{
	models.PQCAlgorithmMLDSA44: {
		generate: func() ([]byte, []byte, error) {
			pk, sk, err := mldsa44.GenerateKey(rand.Reader)
			if err != nil {
				return nil, nil, err
			}
			return pk.Bytes(), sk.Bytes(), nil
		},
		sign: func(skBytes, msg, ctx []byte) ([]byte, error) {
			var sk mldsa44.PrivateKey
			if err := sk.UnmarshalBinary(skBytes); err != nil {
				return nil, err
			}
			sig := make([]byte, mldsa44.SignatureSize)
			return sig, mldsa44.SignTo(&sk, msg, ctx, true, sig)
		},
		verify: func(pkBytes, msg, ctx, sig []byte) bool {
			var pk mldsa44.PublicKey
			return pk.UnmarshalBinary(pkBytes) == nil && mldsa44.Verify(&pk, msg, ctx, sig)
		},
	},
	models.PQCAlgorithmMLDSA65: {
		generate: func() ([]byte, []byte, error) {
			pk, sk, err := mldsa65.GenerateKey(rand.Reader)
			if err != nil {
				return nil, nil, err
			}
			return pk.Bytes(), sk.Bytes(), nil
		},
		sign: func(skBytes, msg, ctx []byte) ([]byte, error) {
			var sk mldsa65.PrivateKey
			if err := sk.UnmarshalBinary(skBytes); err != nil {
				return nil, err
			}
			sig := make([]byte, mldsa65.SignatureSize)
			return sig, mldsa65.SignTo(&sk, msg, ctx, true, sig)
		},
		verify: func(pkBytes, msg, ctx, sig []byte) bool {
			var pk mldsa65.PublicKey
			return pk.UnmarshalBinary(pkBytes) == nil && mldsa65.Verify(&pk, msg, ctx, sig)
		},
	},
	models.PQCAlgorithmMLDSA87: {
		generate: func() ([]byte, []byte, error) {
			pk, sk, err := mldsa87.GenerateKey(rand.Reader)
			if err != nil {
				return nil, nil, err
			}
			return pk.Bytes(), sk.Bytes(), nil
		},
		sign: func(skBytes, msg, ctx []byte) ([]byte, error) {
			var sk mldsa87.PrivateKey
			if err := sk.UnmarshalBinary(skBytes); err != nil {
				return nil, err
			}
			sig := make([]byte, mldsa87.SignatureSize)
			return sig, mldsa87.SignTo(&sk, msg, ctx, true, sig)
		},
		verify: func(pkBytes, msg, ctx, sig []byte) bool {
			var pk mldsa87.PublicKey
			return pk.UnmarshalBinary(pkBytes) == nil && mldsa87.Verify(&pk, msg, ctx, sig)
		},
	},
}

func lookupPQCAlgorithm(name string) (pqcAlgorithm, error) {
	algorithm, ok := pqcAlgorithms[name]
	if !ok {
		return pqcAlgorithm{}, fmt.Errorf("unsupported PQC algorithm %q", name)
	}
	return algorithm, nil
}
