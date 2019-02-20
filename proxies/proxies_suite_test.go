package proxies_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProxies(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxies Suite")
}
