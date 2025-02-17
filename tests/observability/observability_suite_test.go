//go:build !utest

package observability

import (
	"flag"
	"fmt"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/test-network-function/cnfcert-tests-verification/tests/globalhelper"
	tshelper "github.com/test-network-function/cnfcert-tests-verification/tests/observability/helper"
	tsparams "github.com/test-network-function/cnfcert-tests-verification/tests/observability/parameters"
	_ "github.com/test-network-function/cnfcert-tests-verification/tests/observability/tests"
)

func TestObservability(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	_ = flag.Lookup("logtostderr").Value.Set("true")
	_ = flag.Lookup("v").Value.Set(globalhelper.GetConfiguration().General.VerificationLogLevel)
	_, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = globalhelper.GetConfiguration().GetReportPath(currentFile)

	RegisterFailHandler(Fail)
	RunSpecs(t, "CNFCert observability tests", reporterConfig)
}

var _ = SynchronizedBeforeSuite(func() {

	By(fmt.Sprintf("Create %s namespace", tsparams.TestNamespace))
	err := globalhelper.CreateNamespace(tsparams.TestNamespace)
	Expect(err).ToNot(HaveOccurred())

	By("Define TNF config file")
	err = globalhelper.DefineTnfConfig(
		[]string{tsparams.TestNamespace},
		tshelper.GetTnfTargetPodLabelsSlice(),
		[]string{},
		[]string{},
		[]string{tsparams.CrdSuffix1, tsparams.CrdSuffix2})
	Expect(err).ToNot(HaveOccurred())
}, func() {})

var _ = SynchronizedAfterSuite(func() {}, func() {
	By(fmt.Sprintf("Remove %s namespace", tsparams.TestNamespace))
	err := globalhelper.DeleteNamespaceAndWait(tsparams.TestNamespace, tsparams.NsResourcesDeleteTimeoutMins)
	Expect(err).ToNot(HaveOccurred())
})
