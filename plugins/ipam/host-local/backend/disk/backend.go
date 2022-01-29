// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package disk

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend"
)

const lastIPFilePrefix = "last_reserved_ip."
const LineBreak = "\r\n"

var defaultDataDir = "/var/lib/cni/networks"

// Store is a simple disk-backed store that creates one file per IP
// address in a given directory. The contents of the file are the container ID.
type Store struct {
	*FileLock
	dataDir string
}

// Store implements the Store interface
var _ backend.Store = &Store{}

func New(network, dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	dir := filepath.Join(dataDir, network)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lk, err := NewFileLock(dir)
	if err != nil {
		return nil, err
	}
	return &Store{lk, dir}, nil
}

func (s *Store) Reserve(id string, ifname string, ip net.IP, rangeID string) (bool, error) {
	fname := GetEscapedPath(s.dataDir, ip.String())

	f, err := os.OpenFile(fname, os.O_RDWR|os.O_EXCL|os.O_CREATE, 0644)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(strings.TrimSpace(id) + LineBreak + ifname); err != nil {
		f.Close()
		os.Remove(fname)
		return false, err
	}
	if err := f.Close(); err != nil {
		os.Remove(fname)
		return false, err
	}
	// store the reserved ip in lastIPFile
	ipfile := GetEscapedPath(s.dataDir, lastIPFilePrefix+rangeID)
	err = ioutil.WriteFile(ipfile, []byte(ip.String()), 0644)
	if err != nil {
		return false, err
	}
	return true, nil
}

// LastReservedIP returns the last reserved IP if exists
func (s *Store) LastReservedIP(rangeID string) (net.IP, error) {
	ipfile := GetEscapedPath(s.dataDir, lastIPFilePrefix+rangeID)
	data, err := ioutil.ReadFile(ipfile)
	if err != nil {
		return nil, err
	}
	return net.ParseIP(string(data)), nil
}

func (s *Store) Release(ip net.IP) error {
	return os.Remove(GetEscapedPath(s.dataDir, ip.String()))
}

func (s *Store) FindByKey(id string, ifname string, match string) (bool, error) {
	found := false

	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == match {
			found = true
		}
		return nil
	})
	return found, err

}

func (s *Store) FindByID(id string, ifname string) bool {
	s.Lock()
	defer s.Unlock()

	found := false
	match := strings.TrimSpace(id) + LineBreak + ifname
	found, err := s.FindByKey(id, ifname, match)

	// Match anything created by this id
	if !found && err == nil {
		match := strings.TrimSpace(id)
		found, err = s.FindByKey(id, ifname, match)
	}

	return found
}

func (s *Store) ReleaseByKey(id string, ifname string, match string) (bool, error) {
	found := false
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == match {
			if err := os.Remove(path); err != nil {
				return nil
			}
			found = true
		}
		return nil
	})
	return found, err

}

// N.B. This function eats errors to be tolerant and
// release as much as possible
func (s *Store) ReleaseByID(id string, ifname string) error {
	found := false
	match := strings.TrimSpace(id) + LineBreak + ifname
	found, err := s.ReleaseByKey(id, ifname, match)

	// For backwards compatibility, look for files written by a previous version
	if !found && err == nil {
		match := strings.TrimSpace(id)
		found, err = s.ReleaseByKey(id, ifname, match)
	}
	return err
}

// GetByID returns the IPs which have been allocated to the specific ID
func (s *Store) GetByID(id string, ifname string) []net.IP {
	var ips []net.IP

	match := strings.TrimSpace(id) + LineBreak + ifname
	// matchOld for backwards compatibility
	matchOld := strings.TrimSpace(id)

	// walk through all ips in this network to get the ones which belong to a specific ID
	_ = filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == match || strings.TrimSpace(string(data)) == matchOld {
			_, ipString := filepath.Split(path)
			if ip := net.ParseIP(ipString); ip != nil {
				ips = append(ips, ip)
			}
		}
		return nil
	})

	return ips
}

func GetEscapedPath(dataDir string, fname string) string {
	if runtime.GOOS == "windows" {
		fname = strings.Replace(fname, ":", "_", -1)
	}
	return filepath.Join(dataDir, fname)
}

// edge k8s: HasReservedIP verify the pod already had reserved ip or not.
// and return the reserved ip on the other hand.
func (s *Store) HasReservedIP(podNs, podName string) (bool, net.IP) {
	ip := net.IP{}
	if len(podName) == 0 {
		return false, ip
	}

	// Pod, ip mapping info are recorded with file name: PodIP_PodNs_PodName
	podIPNsNameFileName, err := s.findPodFileName("", podNs, podName)
	if err != nil {
		return false, ip
	}

	if len(podIPNsNameFileName) != 0 {
		ipStr, ns, name := resolvePodFileName(podIPNsNameFileName)
		if ns == podNs && name == podName {
			ip = net.ParseIP(ipStr)
			if ip != nil {
				return true, ip
			}
		}
	}

	return false, ip
}

// edge k8s: ReservePodInfo create podName file for storing ip or update ip file with container id and ifname
// in terms of podIPIsExist
func (s *Store) ReservePodInfo(id, ifname string, ip net.IP, podNs, podName string, podIPIsExist bool) (bool, error) {
	if podIPIsExist {
		// pod Ns/Name file is exist, update ip file with new container id and ifname.
		fname := GetEscapedPath(s.dataDir, ip.String())
		err := ioutil.WriteFile(fname, []byte(strings.TrimSpace(id)+LineBreak+ifname), 0644)
		if err != nil {
			return false, err
		}
	} else {
		// for new pod, create a new file named "ip_PodIP_PodNs_PodName",
		// if there is already file named with prefix "ip_PodIP_", rename the old file with new PodNs and PodName.
		if len(podName) != 0 {
			podIPNsNameFile := GetEscapedPath(s.dataDir, podFileName(ip.String(), podNs, podName))
			podIPNsNameFileName, err := s.findPodFileName(ip.String(), "", "")
			if err != nil {
				return false, err
			}

			if len(podIPNsNameFileName) != 0 {
				oldPodIPNsNameFile := GetEscapedPath(s.dataDir, podIPNsNameFileName)
				err = os.Rename(oldPodIPNsNameFile, podIPNsNameFile)
				if err != nil {
					return false, err
				}
				return true, nil
			}

			err = ioutil.WriteFile(podIPNsNameFile, []byte{}, 0644)
			if err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func podFileName(ip, ns, name string) string {
	if len(ip) != 0 && len(ns) != 0 {
		return fmt.Sprintf("ip_%s_%s_%s", ip, ns, name)
	}

	return name
}

func resolvePodFileName(fName string) (ip, ns, name string) {
	parts := strings.Split(fName, "_")
	if len(parts) == 4 {
		ip = parts[1]
		ns = parts[2]
		name = parts[3]
	}

	return
}

func (s *Store) findPodFileName(ip, ns, name string) (string, error) {
	var pattern string
	switch {
	case len(ip) != 0:
		pattern = fmt.Sprintf("ip_%s_*", ip)
	case len(ns) != 0 && len(name) != 0:
		pattern = fmt.Sprintf("ip_*_%s_%s", ns, name)
	default:
		return "", nil
	}
	pattern = GetEscapedPath(s.dataDir, pattern)

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
