package allocator

import (
	"net"

	current "github.com/containernetworking/cni/pkg/types/100"

	db "github.com/containernetworking/plugins/pkg/database"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/containernetworking/plugins/pkg/utils/log"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/disk"
	"github.com/sirupsen/logrus"
)

const (
	defaultLogDir  = "/var/log/cni"
	defaultLogName = "host-local.log"
)

// GetIP allocates an IP or used reserved IP for specified pod
func (a *IPAllocator) GetIP(network, dataDir, envArgs string, id, ifname string, requestedIP net.IP) (*current.IPConfig, error) {
	a.store.Lock()
	defer a.store.Unlock()

	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	podNS, podName, err := utils.ResolvePodNSAndNameFromEnvArgs(envArgs)
	if err != nil {
		log.Errorf("failed to get pod ns/name from env args: %s err %s", envArgs, err)
	}
	if len(podName) == 0 {
		return a.Get(id, ifname, requestedIP)
	}

	knownIP := getIP(*a.rangeset, podNS, podName, network, dataDir)
	if knownIP == nil {
		knownIP = requestedIP
	}

	ipCfg, err := a.Get(id, ifname, knownIP)
	if err != nil {
		if knownIP != nil {
			log.Errorf("the pod %s/%s failed to use reserved IP %s : %s", podNS, podName, knownIP.String(), err)
		}
		return ipCfg, err
	}

	saveIP(podNS, podName, network, dataDir, ipCfg.Address.IP)
	return ipCfg, nil
}

func getIP(rangeset RangeSet, podNS, podName string, network, dataDir string) net.IP {
	if len(podName) == 0 {
		return nil
	}

	err := db.OpenDB(network, dataDir, db.PluginHostLocal)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return nil
	}
	defer db.CloseDB()

	var (
		reservedIP db.ReservedIP
		knownIP    net.IP
		isIPv4     bool
	)
	// the rangeset has already verified by RangeSet's Canonicalize method during loading IPAM config
	// to ensure the address families are uniform
	isIPv4 = rangeset[0].Subnet.IP.To4() != nil

	reservedIP, err = db.GetReservedIP(podNS, podName)
	switch {
	case err == nil:
		if isIPv4 {
			knownIP = net.ParseIP(reservedIP.IPv4)
		} else {
			knownIP = net.ParseIP(reservedIP.IPv6)
		}
	case db.IsNotFoundErr(err):
		return nil
	default:
		log.Errorf("failed to get pod %s/%s reserved IP: %s", podNS, podName, err)
	}

	return knownIP
}

func saveIP(podNS, podName string, network, dataDir string, ip net.IP) {
	if len(podName) == 0 {
		return
	}

	err := db.OpenDB(network, dataDir, db.PluginHostLocal)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	reservedIP, err := db.GetReservedIP(podNS, podName)
	if err != nil && !db.IsNotFoundErr(err) {
		log.Errorf("failed to get pod %s/%s reserved IP: %s", podNS, podName, err)
		return
	}

	isIPv4 := ip.To4() != nil
	reservedIP.Namespace = podNS
	reservedIP.Name = podName
	reservedIP.Deleted = false
	if isIPv4 {
		reservedIP.IPv4 = ip.String()
	} else {
		reservedIP.IPv6 = ip.String()
	}
	err = db.ReserveIP(&reservedIP)
	if err != nil {
		log.Errorf("failed to save pod %s/%s IP: %s", podNS, podName, err)
	}
}

func ReleaseExpiredIPs(store *disk.Store, network, dataDir string, expirationDays int) {
	if expirationDays == 0 {
		return
	}

	store.Lock()
	defer store.Unlock()

	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	err := db.OpenDB(network, dataDir, db.PluginHostLocal)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	err = db.PurgeExpiredIPs(expirationDays)
	if err != nil {
		log.Errorf("failed to purge expired IPs: %s", err)
	}
}

func markDeletedIP(network, dataDir string, envArgs string) {
	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	podNS, podName, err := utils.ResolvePodNSAndNameFromEnvArgs(envArgs)
	if err != nil {
		log.Errorf("failed to get pod ns/name from env args: %s err %s", envArgs, err)
	}
	if len(podName) == 0 {
		return
	}

	err = db.OpenDB(network, dataDir, db.PluginHostLocal)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	var reservedIP db.ReservedIP
	reservedIP, err = db.GetReservedIP(podNS, podName)
	switch {
	case err == nil:
		reservedIP.Deleted = true
		err = db.ReserveIP(&reservedIP)
		if err != nil {
			log.Errorf("failed to save pod %s/%s IP: %s", podNS, podName, err)
		}
	case db.IsNotFoundErr(err):
		// do nothing
	default:
		log.Errorf("failed to get pod %s/%s reserved IP: %s", podNS, podName, err)
	}
}
