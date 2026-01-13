package circuitbreaker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCircuitbreaker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Circuitbreaker Suite")
}
