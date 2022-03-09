package main

import (
	"net"

	"github.com/sirupsen/logrus"

	db "github.com/containernetworking/plugins/pkg/database"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/containernetworking/plugins/pkg/utils/log"
)

const (
	defaultLogDir  = "/var/log/cni"
	defaultLogName = "bridge.log"
)

func getReservedMAC(lock *FileLock, netConf *NetConf, envArgs string) {
	if len(netConf.mac) > 0 {
		// already get mac from MacEnvArgs.MAC
		return
	}

	lock.Lock()
	defer lock.Unlock()

	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	podNS, podName, err := utils.ResolvePodNSAndNameFromEnvArgs(envArgs)
	if err != nil {
		log.Errorf("failed to get pod ns/name from env args: %s", err)
	}

	if len(podName) == 0 {
		return
	}

	err = db.OpenDB(netConf.Name, "", db.PluginBridge)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	expirationDays := netConf.ReservedMACDays
	if expirationDays > 0 {
		err = db.PurgeExpiredMACs(expirationDays)
		if err != nil {
			log.Errorf("failed to purge expired macs: %s", err)
		}
	}

	var reservedMAC db.ReservedMAC
	reservedMAC, err = db.GetReservedMAC(podNS, podName)
	if err != nil && !db.IsNotFoundErr(err) {
		log.Errorf("failed to get pod %s/%s reserved mac: %s", podNS, podName, err)
		return
	}

	if reservedMAC.MAC == "" {
		return
	}

	_, err = net.ParseMAC(reservedMAC.MAC)
	if err != nil {
		log.Errorf("failed to parse the MAC of pod %s/%s: %s, reserved mac is %s", podNS, podName, err, reservedMAC.MAC)
		return
	}

	netConf.mac = reservedMAC.MAC
}

func saveReservedMAC(lock *FileLock, network, envArgs string, containerMAC string) {
	lock.Lock()
	defer lock.Unlock()

	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	podNS, podName, err := utils.ResolvePodNSAndNameFromEnvArgs(envArgs)
	if err != nil {
		log.Errorf("failed to get pod ns/name from env args: %s", err)
	}

	if len(podName) == 0 {
		return
	}

	err = db.OpenDB(network, "", db.PluginBridge)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	reservedMAC, err := db.GetReservedMAC(podNS, podName)
	if err != nil && !db.IsNotFoundErr(err) {
		log.Errorf("failed to get pod %s/%s reserved mac: %s", podNS, podName, err)
		return
	}

	reservedMAC.Namespace = podNS
	reservedMAC.Name = podName
	reservedMAC.MAC = containerMAC
	reservedMAC.Deleted = false
	err = db.ReserveMAC(&reservedMAC)
	if err != nil {
		log.Errorf("failed to save pod %s/%s mac: %s", podNS, podName, err)
	}
}

func releaseMAC(network, envArgs string, expirationDays int) {
	lock, err := NewBridgeFileLock(network, defaultDataDir)
	if err != nil {
		return
	}
	defer lock.Close()

	lock.Lock()
	defer lock.Unlock()

	log.Init(defaultLogDir, defaultLogName, logrus.ErrorLevel)
	defer log.Close()

	podNS, podName, err := utils.ResolvePodNSAndNameFromEnvArgs(envArgs)
	if err != nil {
		log.Errorf("failed to get pod ns/name from env args: %s", err)
	}

	err = db.OpenDB(network, "", db.PluginBridge)
	if err != nil {
		log.Errorf("failed to open database: %s", err)
		return
	}
	defer db.CloseDB()

	if expirationDays > 0 {
		err = db.PurgeExpiredMACs(expirationDays)
		if err != nil {
			log.Errorf("failed to purge expired macs: %s", err)
		}
	}

	if len(podName) == 0 {
		return
	}

	var reservedMAC db.ReservedMAC
	reservedMAC, err = db.GetReservedMAC(podNS, podName)
	switch {
	case err == nil:
		reservedMAC.Deleted = true
		err = db.ReserveMAC(&reservedMAC)
		if err != nil {
			log.Errorf("failed to save pod %s/%s mac: %s", podNS, podName, err)
		}
	case db.IsNotFoundErr(err):
		// do nothing
	default:
		log.Errorf("failed to get pod %s/%s reserved mac: %s", podNS, podName, err)
	}
}
