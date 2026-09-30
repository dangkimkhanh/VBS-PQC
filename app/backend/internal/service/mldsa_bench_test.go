package service

// ML-DSA cost per parameter set, as reported in the thesis:
//   go test ./internal/service -run '^$' -bench MLDSA -benchtime 2s

import (
	"crypto/rand"
	"testing"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
)

var benchMessage = make([]byte, 700) // about the size of a credential manifest
var benchContext = []byte(models.PQCSignatureContext)

func BenchmarkMLDSA44Keygen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mldsa44.GenerateKey(rand.Reader)
	}
}
func BenchmarkMLDSA65Keygen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mldsa65.GenerateKey(rand.Reader)
	}
}
func BenchmarkMLDSA87Keygen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mldsa87.GenerateKey(rand.Reader)
	}
}

func BenchmarkMLDSA44Sign(b *testing.B) {
	_, sk, _ := mldsa44.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa44.SignatureSize)
	for i := 0; i < b.N; i++ {
		_ = mldsa44.SignTo(sk, benchMessage, benchContext, true, sig)
	}
}
func BenchmarkMLDSA65Sign(b *testing.B) {
	_, sk, _ := mldsa65.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa65.SignatureSize)
	for i := 0; i < b.N; i++ {
		_ = mldsa65.SignTo(sk, benchMessage, benchContext, true, sig)
	}
}
func BenchmarkMLDSA87Sign(b *testing.B) {
	_, sk, _ := mldsa87.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa87.SignatureSize)
	for i := 0; i < b.N; i++ {
		_ = mldsa87.SignTo(sk, benchMessage, benchContext, true, sig)
	}
}

func BenchmarkMLDSA44Verify(b *testing.B) {
	pk, sk, _ := mldsa44.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa44.SignatureSize)
	_ = mldsa44.SignTo(sk, benchMessage, benchContext, true, sig)
	for i := 0; i < b.N; i++ {
		mldsa44.Verify(pk, benchMessage, benchContext, sig)
	}
}
func BenchmarkMLDSA65Verify(b *testing.B) {
	pk, sk, _ := mldsa65.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa65.SignatureSize)
	_ = mldsa65.SignTo(sk, benchMessage, benchContext, true, sig)
	for i := 0; i < b.N; i++ {
		mldsa65.Verify(pk, benchMessage, benchContext, sig)
	}
}
func BenchmarkMLDSA87Verify(b *testing.B) {
	pk, sk, _ := mldsa87.GenerateKey(rand.Reader)
	sig := make([]byte, mldsa87.SignatureSize)
	_ = mldsa87.SignTo(sk, benchMessage, benchContext, true, sig)
	for i := 0; i < b.N; i++ {
		mldsa87.Verify(pk, benchMessage, benchContext, sig)
	}
}

func TestSizes(t *testing.T) {
	t.Logf("44 pk=%d sk=%d sig=%d", mldsa44.PublicKeySize, mldsa44.PrivateKeySize, mldsa44.SignatureSize)
	t.Logf("65 pk=%d sk=%d sig=%d", mldsa65.PublicKeySize, mldsa65.PrivateKeySize, mldsa65.SignatureSize)
	t.Logf("87 pk=%d sk=%d sig=%d", mldsa87.PublicKeySize, mldsa87.PrivateKeySize, mldsa87.SignatureSize)
}
