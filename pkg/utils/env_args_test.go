package utils

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	testNamespace = "testNS"
	testName      = "testName"
)

func generateTestArgs(podNS, podName string) string {
	envArgs := ""
	args := [][2]string{
		{"IgnoreUnknown", "1"},
		{"K8S_POD_NAMESPACE", podNS},
		{"K8S_POD_NAME", podName},
		{"K8S_POD_INFRA_CONTAINER_ID", "123456"},
	}
	for i, kv := range args {
		if i > 0 {
			envArgs += ";"
		}
		envArgs += kv[0] + "=" + kv[1]
	}
	return envArgs
}

var _ = Describe("EnvArgs", func() {
	var envArgs string
	When("not exist args", func() {
		BeforeEach(func() {
			envArgs = ""
		})
		It("should be no error", func() {
			podNS, podName, err := ResolvePodNSAndNameFromEnvArgs(envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(podNS).To(Equal(""))
			Expect(podName).To(Equal(""))
		})
	})

	When("exist args", func() {
		BeforeEach(func() {
			envArgs = generateTestArgs(testNamespace, testName)
		})

		It("should return podNS and podName", func() {
			podNS, podName, err := ResolvePodNSAndNameFromEnvArgs(envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(podNS).To(Equal(testNamespace))
			Expect(podName).To(Equal(testName))
		})
	})

	When("podNS and podName total length over 230", func() {
		BeforeEach(func() {
			tmpNS, tmpName := "", ""
			for i := 0; i < 200; i++ {
				tmpNS += "x"
				tmpName += "x"
			}
			envArgs = generateTestArgs(tmpNS, tmpName)
		})

		It("should be error with length limit", func() {
			_, _, err := ResolvePodNSAndNameFromEnvArgs(envArgs)
			Expect(err.Error()).To(ContainSubstring("limit"))
		})
	})
})
