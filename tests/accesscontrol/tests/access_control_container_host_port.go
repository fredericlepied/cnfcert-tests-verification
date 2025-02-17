package accesscontrol

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tshelper "github.com/test-network-function/cnfcert-tests-verification/tests/accesscontrol/helper"
	tsparams "github.com/test-network-function/cnfcert-tests-verification/tests/accesscontrol/parameters"
	"github.com/test-network-function/cnfcert-tests-verification/tests/globalhelper"
	"github.com/test-network-function/cnfcert-tests-verification/tests/globalparameters"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Access-control container-host-port,", func() {
	var randomNamespace string
	var origReportDir string
	var origTnfConfigDir string

	BeforeEach(func() {
		// Create random namespace and keep original report and TNF config directories
		randomNamespace, origReportDir, origTnfConfigDir = globalhelper.BeforeEachSetupWithRandomNamespace(
			tsparams.TestAccessControlNameSpace)

		By("Define tnf config file")
		err := globalhelper.DefineTnfConfig(
			[]string{randomNamespace},
			[]string{tsparams.TestPodLabel},
			[]string{},
			[]string{},
			[]string{})
		Expect(err).ToNot(HaveOccurred(), "error defining tnf config file")
	})

	AfterEach(func() {
		globalhelper.AfterEachCleanupWithRandomNamespace(randomNamespace, origReportDir, origTnfConfigDir, tsparams.Timeout)
	})

	// 63884
	It("one deployment, one pod, one container not declaring host port", func() {
		By("Define deployment with container without host port")
		dep, err := tshelper.DefineDeployment(1, 1, "acdeployment", randomNamespace)
		Expect(err).ToNot(HaveOccurred())

		err = globalhelper.CreateAndWaitUntilDeploymentIsReady(dep, tsparams.Timeout)
		Expect(err).ToNot(HaveOccurred())

		By("Assert deployment has no host port configured")
		runningDeployment, err := globalhelper.GetRunningDeployment(dep.Namespace, dep.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(runningDeployment.Spec.Template.Spec.Containers[0].Ports).To(BeEmpty())

		By("Start test")
		err = globalhelper.LaunchTests(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).ToNot(HaveOccurred())

		By("Verify test case status in Junit and Claim reports")
		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalparameters.TestCasePassed)
		Expect(err).ToNot(HaveOccurred())
	})

	// 63885
	It("one deployment, one pod, one container declaring host port [negative]", func() {
		ports := []corev1.ContainerPort{{ContainerPort: 22223, HostPort: 22222}}

		By("Define deployment with container declaring host port")
		dep, err := tshelper.DefineDeploymentWithContainerPorts("acdeployment", randomNamespace, 1, ports)
		Expect(err).ToNot(HaveOccurred())

		err = globalhelper.CreateAndWaitUntilDeploymentIsReady(dep, tsparams.Timeout)
		Expect(err).ToNot(HaveOccurred())

		By("Assert deployment has container has container/host port configured")
		runningDeployment, err := globalhelper.GetRunningDeployment(dep.Namespace, dep.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(runningDeployment.Spec.Template.Spec.Containers[0].Ports).ToNot(BeEmpty())
		Expect(runningDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(22223)))
		Expect(runningDeployment.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(22222)))

		By("Start test")
		err = globalhelper.LaunchTests(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).To(HaveOccurred())

		By("Verify test case status in Junit and Claim reports")
		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalparameters.TestCaseFailed)
		Expect(err).ToNot(HaveOccurred())
	})

	// 63886
	It("one deployment, one pod, two containers, neither declaring host port", func() {
		ports := []corev1.ContainerPort{{ContainerPort: 22222}, {ContainerPort: 22223}}

		By("Define deployment with containers not declaring host port")
		dep, err := tshelper.DefineDeploymentWithContainerPorts("acdeployment", randomNamespace, 1, ports)
		Expect(err).ToNot(HaveOccurred())

		err = globalhelper.CreateAndWaitUntilDeploymentIsReady(dep, tsparams.Timeout)
		Expect(err).ToNot(HaveOccurred())

		By("Assert deployment does not have host port configured")
		runningDeployment, err := globalhelper.GetRunningDeployment(dep.Namespace, dep.Name)
		Expect(err).ToNot(HaveOccurred())
		for _, container := range runningDeployment.Spec.Template.Spec.Containers {
			for _, port := range container.Ports {
				Expect(port.HostPort).To(BeZero())
				Expect(port.ContainerPort).Should(BeElementOf([]int32{22222, 22223}))
			}
		}

		By("Start test")
		err = globalhelper.LaunchTests(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).ToNot(HaveOccurred())

		By("Verify test case status in Junit and Claim reports")
		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalparameters.TestCasePassed)
		Expect(err).ToNot(HaveOccurred())
	})

	// 63887
	It("one deployment, one pod, two containers, one declaring host port [negative]", func() {
		ports := []corev1.ContainerPort{{ContainerPort: 22221}, {ContainerPort: 22222, HostPort: 22223}}

		By("Define deployment with one container declaring host port")
		dep, err := tshelper.DefineDeploymentWithContainerPorts("acdeployment", randomNamespace, 1, ports)
		Expect(err).ToNot(HaveOccurred())

		err = globalhelper.CreateAndWaitUntilDeploymentIsReady(dep, tsparams.Timeout)
		Expect(err).ToNot(HaveOccurred())

		By("Assert deployment has container/host port configured")
		runningDeployment, err := globalhelper.GetRunningDeployment(dep.Namespace, dep.Name)
		Expect(err).ToNot(HaveOccurred())
		for _, container := range runningDeployment.Spec.Template.Spec.Containers {
			for _, port := range container.Ports {
				if port.ContainerPort == 22222 {
					Expect(port.HostPort).To(Equal(int32(22223)))
				} else if port.ContainerPort == 22221 {
					Expect(port.HostPort).To(BeZero())
				}
			}
		}

		By("Start test")
		err = globalhelper.LaunchTests(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalhelper.ConvertSpecNameToFileName(CurrentSpecReport().FullText()))
		Expect(err).To(HaveOccurred())

		By("Verify test case status in Junit and Claim reports")
		err = globalhelper.ValidateIfReportsAreValid(
			tsparams.TestCaseNameAccessControlContainerHostPort,
			globalparameters.TestCaseFailed)
		Expect(err).ToNot(HaveOccurred())
	})

})
