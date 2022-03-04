package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/disk"
)

const deletionTag = "deleted"

var defaultDataDir = "/var/lib/cni/networks"

// Store is a simple disk-backed store that creates one file per mac_MAC
// address in a given directory.
type Store struct {
	*disk.FileLock
	dataDir string
}

func New(network, dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	network = fmt.Sprintf("%s_macs", network)
	dir := filepath.Join(dataDir, network)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lk, err := disk.NewFileLock(dir)
	if err != nil {
		return nil, err
	}
	return &Store{lk, dir}, nil
}

// edge k8s: hasReservedMAC verify the pod already had reserved MAC or not.
// and return the reserved mac on the other hand.
func (s *Store) hasReservedMAC(podNS, podName string) (net.HardwareAddr, error) {
	if len(podName) == 0 {
		return nil, nil
	}

	// Pod, mac mapping info are recorded with file name: mac_PodMAC_PodNs_PodName
	podFileName, err := s.findPodFileName("", podNS, podName)
	if err != nil {
		return nil, err
	}

	if len(podFileName) != 0 {
		mac, ns, name := resolvePodFileName(podFileName)
		if ns == podNS && name == podName {
			hw, err := net.ParseMAC(mac)
			if err != nil {
				return nil, nil
			}
			return hw, nil
		}
	}

	return nil, nil
}

// podFileName mac_PodMAC_PodNs_PodName
func podFileName(mac, ns, name string) string {
	if len(mac) != 0 && len(ns) != 0 {
		// the mac format is c6-8d-0b-db-4e-83 for getting escaped path in windows OS
		mac = strings.ReplaceAll(mac, ":", "-")
		return fmt.Sprintf("mac_%s_%s_%s", mac, ns, name)
	}

	return name
}

// mac_podMac_podNs_podName
func resolvePodFileName(fName string) (mac, ns, name string) {
	parts := strings.Split(fName, "_")
	if len(parts) == 4 {
		mac = parts[1]
		ns = parts[2]
		name = parts[3]
	}

	return
}

func (s *Store) findPodFileName(mac, ns, name string) (string, error) {
	var pattern string
	switch {
	case len(mac) != 0:
		mac = strings.ReplaceAll(mac, ":", "-")
		pattern = fmt.Sprintf("mac_%s_*", mac)
	case len(ns) != 0 && len(name) != 0:
		pattern = fmt.Sprintf("mac_*_%s_%s", ns, name)
	default:
		return "", nil
	}
	pattern = disk.GetEscapedPath(s.dataDir, pattern)

	podFiles, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(podFiles) == 1 {
		_, fName := filepath.Split(podFiles[0])
		if strings.Count(fName, "_") == 3 {
			return fName, nil
		}
	}

	return "", nil
}

// edge k8s: reservePodInfo create podName file for storing mac
// in terms of podMacIsExist
func (s *Store) reservePodInfo(mac, podNs, podName string) (bool, error) {
	if len(podName) == 0 {
		return false, nil
	}

	if len(mac) == 0 {
		// delete pod
		podMacNsNameFileName, err := s.findPodFileName("", podNs, podName)
		if err != nil {
			return false, err
		}
		if len(podMacNsNameFileName) > 0 {
			podMacNsNameFilePath := disk.GetEscapedPath(s.dataDir, podMacNsNameFileName)
			err = ioutil.WriteFile(podMacNsNameFilePath, []byte(deletionTag), 0644)
			if err != nil {
				return false, err
			}
		}

		return true, nil
	}

	// for adding pod, create a new file named "mac_PodMac_PodNs_PodName",
	// if there is already file named with "mac_*_PodNs_PodName", rename the old file with new PodNs and PodName.
	targetPodMACNsNameFile := podFileName(mac, podNs, podName)
	targetPodMACNsNameFilePath := disk.GetEscapedPath(s.dataDir, targetPodMACNsNameFile)
	podMacNsNameFileName, err := s.findPodFileName("", podNs, podName)
	if err != nil {
		return false, err
	}

	if len(podMacNsNameFileName) != 0 && targetPodMACNsNameFile != podMacNsNameFileName {
		oldPodMacNsNameFilePath := disk.GetEscapedPath(s.dataDir, podMacNsNameFileName)
		err = os.Rename(oldPodMacNsNameFilePath, targetPodMACNsNameFilePath)
		if err != nil {
			return false, err
		} else {
			return true, nil
		}
	}

	err = ioutil.WriteFile(targetPodMACNsNameFilePath, []byte{}, 0644)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) GetContainerMAC(podNS, podName string) (string, error) {
	s.Lock()
	defer s.Unlock()

	hw, err := s.hasReservedMAC(podNS, podName)
	if hw == nil || err != nil {
		return "", err
	}
	return hw.String(), nil
}

func (s *Store) SaveContainerMac(mac, podNs, podName string) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.reservePodInfo(mac, podNs, podName)

	return err
}

func (s *Store) RemoveExpiredRecords(pattern string, expirationDays int) error {
	s.Lock()
	defer s.Unlock()

	removeTime := time.Now().Add(-time.Hour * 24 * time.Duration(expirationDays))
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		found, err := regexp.MatchString(pattern, info.Name())
		if !found || err != nil {
			return nil
		}
		if info.ModTime().After(removeTime) {
			return nil
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == deletionTag {
			if err := os.Remove(path); err != nil {
				return nil
			}
		}
		return nil
	})
	return err
}
