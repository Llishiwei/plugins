package utils

import (
	"fmt"
	"strings"
)

// Args: [][2]string{
// {"IgnoreUnknown", "1"},
// {"K8S_POD_NAMESPACE", podNs},
// {"K8S_POD_NAME", podName},
// {"K8S_POD_INFRA_CONTAINER_ID", podSandboxID.ID},
// },
func ResolvePodNSAndNameFromEnvArgs(envArgs string) (string, string, error) {
	var ns, name string
	if envArgs == "" {
		return ns, name, nil
	}

	pairs := strings.Split(envArgs, ";")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return ns, name, fmt.Errorf("ARGS: invalid pair %q", pair)
		}

		if kv[0] == "K8S_POD_NAMESPACE" {
			ns = kv[1]
		} else if kv[0] == "K8S_POD_NAME" {
			name = kv[1]
		}
	}

	if len(ns)+len(name) > 230 {
		return "", "", fmt.Errorf("ARGS: length of pod ns and name exceed the length limit")
	}
	return ns, name, nil
}
