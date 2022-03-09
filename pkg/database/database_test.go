package database

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Database", func() {
	const (
		testNetwork        = "testDatabase"
		testDataDirPattern = "cniDBTestDir"
	)

	Context("when plugin is bridge", func() {
		var (
			testDataDir string
			err         error
			now         = time.Now()
			days        = 2
			macList     = []ReservedMAC{
				{
					MAC: "02:42:af:a3:d8:01",
					BaseModel: BaseModel{
						Namespace: "NS1",
						Name:      "pod1",
						Deleted:   true,
						CreatedAt: now,
						UpdatedAt: now.AddDate(0, 0, -days),
					},
				},
				{
					MAC: "02:42:af:a3:d8:02",
					BaseModel: BaseModel{
						Namespace: "NS2",
						Name:      "pod2",
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
				{
					MAC: "02:42:af:a3:d8:03",
					BaseModel: BaseModel{
						Namespace: "NS3",
						Name:      "pod3",
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
			}
		)
		BeforeEach(func() {
			testDataDir, err = os.MkdirTemp("", testDataDirPattern)
			Expect(err).NotTo(HaveOccurred())
			err = OpenDB(testNetwork, testDataDir, PluginBridge)
			Expect(err).NotTo(HaveOccurred())

			for _, mac := range macList {
				err = ReserveMAC(&mac)
				Expect(err).To(BeNil())
			}
		})

		AfterEach(func() {
			err = CloseDB()
			Expect(err).NotTo(HaveOccurred())

			err = os.RemoveAll(testDataDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be succeed to get reserved mac", func() {
			var reservedMAC ReservedMAC
			reservedMAC, err = GetReservedMAC("NS2", "pod2")
			Expect(err).To(BeNil())
			Expect(reservedMAC.MAC).To(Equal("02:42:af:a3:d8:02"))
		})

		It("should be succeed to delete expired record", func() {
			err = PurgeExpiredMACs(1)
			Expect(err).To(BeNil())
			_, err = GetReservedMAC("NS1", "pod1")
			Expect(IsNotFoundErr(err)).To(BeTrue())
		})
	})

	Context("when plugin is host-local", func() {
		var (
			testDataDir string
			err         error
			now         = time.Now()
			days        = 2
			ipList      = []ReservedIP{
				{
					IPv4: "10.10.10.1",
					BaseModel: BaseModel{
						Namespace: "NS1",
						Name:      "pod1",
						Deleted:   true,
						CreatedAt: now,
						UpdatedAt: now.AddDate(0, 0, -days),
					},
				},
				{
					IPv4: "10.10.10.2",
					BaseModel: BaseModel{
						Namespace: "NS2",
						Name:      "pod2",
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
				{
					IPv4: "10.10.10.3",
					BaseModel: BaseModel{
						Namespace: "NS3",
						Name:      "pod3",
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
			}
		)
		BeforeEach(func() {
			testDataDir, err = os.MkdirTemp("", testDataDirPattern)
			Expect(err).NotTo(HaveOccurred())
			err = OpenDB(testNetwork, testDataDir, PluginHostLocal)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range ipList {
				err = ReserveIP(&ip)
				Expect(err).To(BeNil())
			}
		})

		AfterEach(func() {
			err = CloseDB()
			Expect(err).NotTo(HaveOccurred())

			err = os.RemoveAll(testDataDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be succeed to get reserved ip", func() {
			var reservedIP ReservedIP
			reservedIP, err = GetReservedIP("NS2", "pod2")
			Expect(err).To(BeNil())
			Expect(reservedIP.IPv4).To(Equal("10.10.10.2"))
		})

		It("should be succeed to delete expired record", func() {
			err = PurgeExpiredIPs(1)
			Expect(err).To(BeNil())
			_, err = GetReservedIP("NS1", "pod1")
			Expect(IsNotFoundErr(err)).To(BeTrue())
		})
	})

	Context("when plugin is neither bridge nor host-local", func() {
		var (
			testDataDir string
			err         error
		)
		BeforeEach(func() {
			testDataDir, err = os.MkdirTemp("", testDataDirPattern)
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			err = os.RemoveAll(testDataDir)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should be error with not support plugin", func() {
			err := OpenDB(testNetwork, testDataDir, "dhcp")
			Expect(err.Error()).To(ContainSubstring("not support"))
		})
	})
})
