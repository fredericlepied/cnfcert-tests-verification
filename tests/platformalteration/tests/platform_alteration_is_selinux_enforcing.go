package tests

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/test-network-function/cnfcert-tests-verification/tests/globalhelper"
	"github.com/test-network-function/cnfcert-tests-verification/tests/globalparameters"
	tsparams "github.com/test-network-function/cnfcert-tests-verification/tests/platformalteration/parameters"
	"github.com/test-network-function/cnfcert-tests-verification/tests/utils/daemonset"
)

var _ = Describe("platform-alteration-is-selinux-enforcing", func() {
	var randomNamespace string
	var origReportDir string
	var origTnfConfigDir string

	BeforeEach(func() {
		// Create random namespace and keep original report and TNF config directories
		randomNamespace, origReportDir, origTnfConfigDir = globalhelper.BeforeEachSetupWithRandomNamespace(
			tsparams.PlatformAlterationNamespace)

		By("Define TNF config file")
		err := globalhelper.DefineTnfConfig(
			[]string{randomNamespace},
			[]string{tsparams.TestPodLabel},
			[]string{},
			[]string{},
			[]string{})
		Expect(err).ToNot(HaveOccurred())

		By("If Kind cluster, skip")
		if globalhelper.IsKindCluster() {
			Skip("Kind cluster does not support SELinux")
		}
	})

	AfterEach(func() {
		globalhelper.AfterEachCleanupWithRandomNamespace(randomNamespace, origReportDir, origTnfConfigDir, tsparams.WaitingTime)
	})

	// 51310
	It("SELinux is enforcing on all nodes", func() {
		daemonSet := daemonset.DefineDaemonSet(randomNamespace, globalhelper.GetConfiguration().General.TestImage,
			tsparams.TnfTargetPodLabels, tsparams.TestDaemonSetName)
		daemonset.RedefineWithPrivilegedContainer(daemonSet)
		daemonset.RedefineWithVolumeMount(daemonSet)

		err := globalhelper.CreateAndWaitUntilDaemonSetIsReady(daemonSet, tsparams.WaitingTime)
		Expect(err).ToNot(HaveOccurred())

		podList, err := globalhelper.GetListOfPodsInNamespace(randomNamespace)
		Expect(err).ToNot(HaveOccurred())

		By("Verify that all nodes are running with selinux on enforcing mode")
		for _, pod := range podList.Items {

			buf, err := globalhelper.ExecCommand(pod, []string{"/bin/bash", "-c", tsparams.Getenforce})
			Expect(err).ToNot(HaveOccurred())

			if !strings.Contains(buf.String(), tsparams.Enforcing) {
				_, err = globalhelper.ExecCommand(pod, []string{"/bin/bash", "-c", tsparams.SetEnforce})
				Expect(err).ToNot(HaveOccurred())
			}
		}

		By("Start platform-alteration-is-selinux-enforcing test")
		err = globalhelper.LaunchTests(tsparams.TnfIsSelinuxEnforcingName,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).ToNot(HaveOccurred())

		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TnfIsSelinuxEnforcingName,
			globalparameters.TestCasePassed)
		Expect(err).ToNot(HaveOccurred())
	})

	// 51311
	It("SELinux is permissive on one node [negative]", func() {
		if globalhelper.IsKindCluster() {
			Skip("Kind cluster does not support SELinux")
		}

		Skip("Skipping. Remove this skip when we can detect if SELinux is enabled on the node")

		daemonSet := daemonset.DefineDaemonSet(randomNamespace, globalhelper.GetConfiguration().General.TestImage,
			tsparams.TnfTargetPodLabels, tsparams.TestDaemonSetName)
		daemonset.RedefineWithPrivilegedContainer(daemonSet)
		daemonset.RedefineWithVolumeMount(daemonSet)

		err := globalhelper.CreateAndWaitUntilDaemonSetIsReady(daemonSet, tsparams.WaitingTime)
		Expect(err).ToNot(HaveOccurred())

		podList, err := globalhelper.GetListOfPodsInNamespace(randomNamespace)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(podList.Items)).NotTo(BeZero())

		By("Set SELinux permissive on the node")
		_, err = globalhelper.ExecCommand(podList.Items[0], []string{"/bin/bash", "-c", tsparams.SetPermissive})
		Expect(err).ToNot(HaveOccurred())

		By("Start platform-alteration-is-selinux-enforcing test")
		err = globalhelper.LaunchTests(tsparams.TnfIsSelinuxEnforcingName,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).To(HaveOccurred())

		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TnfIsSelinuxEnforcingName,
			globalparameters.TestCaseFailed)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying SELinux is enforcing on the node")
		_, err = globalhelper.ExecCommand(podList.Items[0], []string{"/bin/bash", "-c", tsparams.SetEnforce})
		Expect(err).ToNot(HaveOccurred())

	})
})
