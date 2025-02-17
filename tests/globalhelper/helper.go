package globalhelper

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/test-network-function/cnfcert-tests-verification/tests/globalparameters"
	"github.com/test-network-function/cnfcert-tests-verification/tests/utils/rbac"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const retryInterval = 5

// ValidateIfReportsAreValid test if report is valid for given test case.
func ValidateIfReportsAreValid(tcName string, tcExpectedStatus string) error {
	glog.V(5).Info("Verify test case status in Junit report")

	junitTestReport, err := OpenJunitTestReport()
	if err != nil {
		return fmt.Errorf("failed to open junit test report, err: %w", err)
	}

	claimReport, err := OpenClaimReport()
	if err != nil {
		return fmt.Errorf("failed to open tnf claim report, err: %w", err)
	}

	err = IsExpectedStatusParamValid(tcExpectedStatus)
	if err != nil {
		return fmt.Errorf("expected status %q is not valid, err: %w", tcExpectedStatus, err)
	}

	isTestCaseInValidStatusInJunitReport := IsTestCasePassedInJunitReport
	isTestCaseInValidStatusInClaimReport := IsTestCasePassedInClaimReport

	if tcExpectedStatus == globalparameters.TestCaseFailed {
		isTestCaseInValidStatusInJunitReport = IsTestCaseFailedInJunitReport
		isTestCaseInValidStatusInClaimReport = IsTestCaseFailedInClaimReport
	}

	if tcExpectedStatus == globalparameters.TestCaseSkipped {
		isTestCaseInValidStatusInJunitReport = IsTestCaseSkippedInJunitReport
		isTestCaseInValidStatusInClaimReport = IsTestCaseSkippedInClaimReport
	}

	glog.V(5).Info("Verify test case status in junit report file")

	isValid, err := isTestCaseInValidStatusInJunitReport(junitTestReport, tcName)
	if !isValid {
		return fmt.Errorf("test case %q is not in expected %q state in junit report. %w", tcName, tcExpectedStatus, err)
	}

	glog.V(5).Info("Verify test case status in claim report file")

	testPassed, err := isTestCaseInValidStatusInClaimReport(tcName, *claimReport)
	if err != nil {
		return fmt.Errorf("failed to get the state of test case %q from the claim report file, err: %w", tcName, err)
	}

	if !testPassed {
		return fmt.Errorf("test case %q is not in expected %q state in claim report", tcName, tcExpectedStatus)
	}

	return nil
}

// DefineTnfConfig creates tnf_config.yml file under tnf config directory.
func DefineTnfConfig(namespaces []string, targetPodLabels []string, targetOperatorLabels []string,
	certifiedContainerInfo []string, crdFilters []string) error {
	tnfConfigFilePath := path.Join(GetConfiguration().General.TnfConfigDir, globalparameters.DefaultTnfConfigFileName)

	configFile, err := os.OpenFile(tnfConfigFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("error opening/creating file %s: %w", tnfConfigFilePath, err)
	}

	defer configFile.Close()
	configFileEncoder := yaml.NewEncoder(configFile)
	tnfConfig := globalparameters.TnfConfig{}

	err = defineTnfNamespaces(&tnfConfig, namespaces)
	if err != nil {
		return fmt.Errorf("failed to create namespaces section in tnf yaml config file: %w", err)
	}

	err = definePodUnderTestLabels(&tnfConfig, targetPodLabels)
	if err != nil {
		return fmt.Errorf("failed to create target pod labels section in tnf yaml config file: %w", err)
	}

	defineOperatorsUnderTestLabels(&tnfConfig, targetOperatorLabels)

	err = defineCertifiedContainersInfo(&tnfConfig, certifiedContainerInfo)
	if err != nil {
		return fmt.Errorf("failed to create certified containers info section in tnf yaml config file: %w", err)
	}

	// CRD filters is an optional field.
	defineCrdFilters(&tnfConfig, crdFilters)

	err = configFileEncoder.Encode(tnfConfig)
	if err != nil {
		return fmt.Errorf("failed to encode tnf yaml config file on %s: %w", tnfConfigFilePath, err)
	}

	glog.V(5).Info(fmt.Sprintf("%s deployed under %s directory",
		globalparameters.DefaultTnfConfigFileName, GetConfiguration().General.TnfConfigDir))

	return nil
}

// IsExpectedStatusParamValid validates if requested test status is valid.
func IsExpectedStatusParamValid(status string) error {
	return validateIfParamInAllowedListOfParams(
		status,
		[]string{globalparameters.TestCaseFailed, globalparameters.TestCasePassed, globalparameters.TestCaseSkipped})
}

// AppendContainersToDeployment appends containers to a deployment.
func AppendContainersToDeployment(deployment *appsv1.Deployment, containersNum int, image string) {
	containerList := &deployment.Spec.Template.Spec.Containers

	for i := 0; i < containersNum; i++ {
		*containerList = append(
			*containerList, corev1.Container{
				Name:    fmt.Sprintf("container%d", i+1),
				Image:   image,
				Command: []string{"/bin/bash", "-c", "sleep INF"},
			})
	}
}

func defineCertifiedContainersInfo(config *globalparameters.TnfConfig, certifiedContainerInfo []string) error {
	if len(certifiedContainerInfo) < 1 {
		// do not add certifiedcontainerinfo to tnf_config at all in this case
		return nil
	}

	for _, certifiedContainerFields := range certifiedContainerInfo {
		repositoryRegistryTagDigest := strings.Split(certifiedContainerFields, ";")

		if len(repositoryRegistryTagDigest) == 1 {
			// certifiedContainerInfo item does not contain separation character
			// use this to add only the Certifiedcontainerinfo field with no sub fields
			var emptyInfo globalparameters.CertifiedContainerRepoInfo
			config.Certifiedcontainerinfo = append(config.Certifiedcontainerinfo, emptyInfo)

			return nil
		}

		if len(repositoryRegistryTagDigest) != 4 {
			return fmt.Errorf(fmt.Sprintf("certified container info %s is invalid", certifiedContainerFields))
		}

		repo := strings.TrimSpace(repositoryRegistryTagDigest[0])
		registry := strings.TrimSpace(repositoryRegistryTagDigest[1])
		tag := strings.TrimSpace(repositoryRegistryTagDigest[2])
		digest := strings.TrimSpace(repositoryRegistryTagDigest[3])

		glog.V(5).Info(fmt.Sprintf("Adding container repository:%s registry:%s to configuration", repo, registry))

		config.Certifiedcontainerinfo = append(config.Certifiedcontainerinfo, globalparameters.CertifiedContainerRepoInfo{
			Repository: repo,
			Registry:   registry,
			Tag:        tag,
			Digest:     digest,
		})
	}

	return nil
}

func defineTnfNamespaces(config *globalparameters.TnfConfig, namespaces []string) error {
	if len(namespaces) < 1 {
		return fmt.Errorf("target namespaces cannot be empty list")
	}

	if config == nil {
		return fmt.Errorf("config struct cannot be nil")
	}

	for _, namespace := range namespaces {
		config.TargetNameSpaces = append(config.TargetNameSpaces, globalparameters.TargetNameSpace{
			Name: namespace,
		})
	}

	return nil
}

func definePodUnderTestLabels(config *globalparameters.TnfConfig, podsUnderTestLabels []string) error {
	if len(podsUnderTestLabels) < 1 {
		return fmt.Errorf("pods under test labels cannot be empty list")
	}

	for _, podsUnderTestLabel := range podsUnderTestLabels {
		prefixNameValue := strings.Split(podsUnderTestLabel, ":")
		if len(prefixNameValue) != 2 {
			return fmt.Errorf(fmt.Sprintf("podUnderTest label %s is invalid", podsUnderTestLabel))
		}

		config.PodsUnderTestLabels = append(config.PodsUnderTestLabels, prefixNameValue[0]+":"+prefixNameValue[1])
	}

	return nil
}

func defineOperatorsUnderTestLabels(config *globalparameters.TnfConfig, operatorsUnderTestLabels []string) {
	if len(operatorsUnderTestLabels) > 0 {
		config.OperatorsUnderTestLabels = append(config.OperatorsUnderTestLabels, operatorsUnderTestLabels...)
	}
}

func defineCrdFilters(config *globalparameters.TnfConfig, crdSuffixes []string) {
	for _, crdSuffix := range crdSuffixes {
		glog.V(5).Info(fmt.Sprintf("Adding crd suffix %s to tnf configuration file", crdSuffix))

		config.TargetCrdFilters = append(config.TargetCrdFilters, globalparameters.TargetCrdFilter{
			NameSuffix: crdSuffix,
		})
	}
}

func validateIfParamInAllowedListOfParams(parameter string, listOfParameters []string) error {
	for _, allowedParameter := range listOfParameters {
		if allowedParameter == parameter {
			return nil
		}
	}

	return fmt.Errorf("parameter %s is not allowed. List of allowed parameters %s", parameter, listOfParameters)
}

// AllowAuthenticatedUsersRunPrivilegedContainers adds all authenticated users to privileged group.
func AllowAuthenticatedUsersRunPrivilegedContainers() error {
	_, err := GetAPIClient().ClusterRoleBindings().Get(
		context.TODO(),
		"system:openshift:scc:privileged",
		metav1.GetOptions{},
	)
	if k8serrors.IsNotFound(err) {
		glog.V(5).Info("RBAC policy is not found")

		roleBind := rbac.DefineClusterRoleBinding(
			*rbac.DefineRbacAuthorizationClusterRoleRef("system:openshift:scc:privileged"),
			*rbac.DefineRbacAuthorizationClusterGroupSubjects([]string{"system:authenticated"}),
		)

		_, err = GetAPIClient().ClusterRoleBindings().Create(context.TODO(), roleBind, metav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(err) {
			glog.V(5).Info("RBAC policy already exists")
		} else if err != nil {
			return fmt.Errorf("failed to create RBAC policy: %w", err)
		}

		glog.V(5).Info("RBAC policy created")

		return nil
	} else if err == nil {
		glog.V(5).Info("RBAC policy detected")
	}

	glog.V(5).Info("error to query RBAC policy")

	return nil
}

// CopyFiles copy file from source to destination.
func CopyFiles(src string, dst string) error {
	originalFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", src, err)
	}

	defer originalFile.Close()

	newFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dst, err)
	}

	defer newFile.Close()

	_, err = io.Copy(newFile, originalFile)
	if err != nil {
		return fmt.Errorf("failed to copy file %s - %s : %w", src, dst, err)
	}

	return nil
}

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func GenerateRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz"

	//nolint:varnamelen
	b := make([]byte, length)

	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}

// Returns true if the cluster is of kind type, otherwise false. Performance
// gains are achievable by invoking the command once, leveraging a
// synchronization mechanism like sync.Once.
func IsKindCluster() bool {
	cmd := exec.Command(
		"oc",
		"cluster-info", "--context", "kind-kind",
		">/dev/null", "2>&1")

	return cmd.Run() == nil
}
