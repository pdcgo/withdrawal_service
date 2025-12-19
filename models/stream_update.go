package models

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/pdcgo/shared/db_models"
)

type ShopeeWdTxType string

const (
	WdTxFromOrder  ShopeeWdTxType = "Penghasilan dari Pesanan"
	WdTxAdjustment ShopeeWdTxType = "Penyesuaian"
	WdTxFund       ShopeeWdTxType = "Penarikan Dana"
)

type ShopeeWdKind string

const (
	StreamTxIn ShopeeWdKind = "Transaksi Masuk"
)

type ShopeeWdStatus string

const (
	ShopeeWdStatusCompleted ShopeeWdStatus = "Transaksi Selesai"
)

type ShopeeWdItem struct {
	TransactionDate time.Time      `xls:"0" xlsdate:"2006-01-02 15:04:05" fallback_xlsdate:"2006-01-02 15:04"`
	Type            ShopeeWdTxType `xls:"1"`
	Kind            ShopeeWdKind   `xls:"4"`
	Description     string         `xls:"2"`
	ExternalOrderID string         `xls:"3"`
	Amount          float64        `xls:"5"`
	Status          ShopeeWdStatus `xls:"6"`
	BalanceAfter    float64        `xls:"7"`
}

func (item *ShopeeWdItem) OrderIDExtractFromDesc() string {
	r := regexp.MustCompile(`[0-9]{6}[0-9A-Z]{8}`)
	return r.FindString(item.Description)
}

func (item *ShopeeWdItem) IsCommisionAdjustment() bool {
	return strings.Contains(item.Description, "Pemotongan biaya komisi")
}

func (item *ShopeeWdItem) IsCompensationAdjustment() bool {
	return strings.Contains(item.Description, "compensation")
}

func (item *ShopeeWdItem) IsLostCompensationAdjustment() bool {
	return strings.Contains(item.Description, "Kompensasi kehilangan")
}
func (item *ShopeeWdItem) TryParseOtherType() (db_models.AdjustmentType, error) {

	if strings.Contains(item.Description, "Pemotongan biaya komisi") {
		return db_models.AdjCommision, nil
	}

	if strings.Contains(item.Description, "compensation") {
		return db_models.AdjCompensation, nil
	}

	if strings.Contains(item.Description, "Kompensasi kehilangan") {
		return db_models.AdjLostCompensation, nil
	}

	if strings.Contains(item.Description, "Penyesuaian Saldo Penjual untuk biaya premi Pesanan") {
		return db_models.AdjPremi, nil
	}

	if strings.Contains(item.Description, "Kompensasi Biaya Kemasan Program Garansi Bebas") {
		return db_models.AdjPackaging, nil
	}

	if strings.Contains(item.Description, "[Penambahan Wallet] Pengembalian Dana dari Order Return") {
		return db_models.AdjReturn, nil
	}

	if strings.Contains(item.Description, "karena terdapat Pengembalian Barang/Dana setelah dana dilepaskan") {
		return db_models.AdjReturn, nil
	}

	if strings.Contains(item.Description, "Penyesuaian Ongkos Kirim Bebas Pengembalian") {
		return db_models.AdjShipping, nil
	}

	if strings.Contains(item.Description, "Penggantian Dana Penuh Barang Hilang") {
		return db_models.AdjLostCompensation, nil
	}

	if strings.Contains(item.Description, "Penggantian Dana Sebagian Barang Hilang") {
		return db_models.AdjLostCompensation, nil
	}

	return db_models.AdjUnknownAdj, errors.New("cannot parse other type")
}

type ShopeeOrderStatus string

const (
	ShopeeOrderStatusCancel ShopeeOrderStatus = "batal"
)

type ShopeeOrderCancel struct {
	ExternalOrderID      string
	Status               ShopeeOrderStatus
	ReturnStatus         string
	ShippingFailedStatus string
	Receipt              string
}

func (item *ShopeeOrderCancel) Marshal(data []string) error {
	if len(data) < 2 {
		return errors.New("data not valid")
	}
	for i, val := range data {
		switch i {
		case 0:
			item.ExternalOrderID = val
		case 1:
			val = strings.ToLower(val)
			item.Status = ShopeeOrderStatus(val)
		case 2:
			item.ReturnStatus = val
		case 3:
			item.ShippingFailedStatus = val
		case 4:
			item.Receipt = val

		}
	}

	return nil
}

type ShopeeReturnStatus string

const (
	ShopeeReturnCancel ShopeeReturnStatus = "pengajuan dibatalkan"
	ShopeeReturnWait   ShopeeReturnStatus = "sedang dalam persetujuan"
	ShopeeReturnReject ShopeeReturnStatus = "banding ditolak"
	ShopeeFundReturn   ShopeeReturnStatus = "data dikembalikan ke pembeli"
)

type ShopeeReturnType string

const (
	ShopeeReturnPartial  ShopeeReturnType = "sebagian pesanan"
	ShopeeReturnComplete ShopeeReturnType = "seluruh pesanan"
)

type Solution string

const (
	ReturnGoodAndRefund Solution = "pengembalian barang dan dana"
)

type ShopeeReturnOrder struct {
	ReturnID        string
	ExternalOrderID string
	Status          ShopeeReturnStatus
	Type            ShopeeReturnType
	Solution        Solution
	Note            string
	Reason          string
	Receipt         string
}

func (item *ShopeeReturnOrder) Marshal(data []string) error {
	if len(data) < 2 {
		return errors.New("data not valid")
	}

	rowmap := map[int]string{}
	for i, d := range data {
		item := d
		rowmap[i] = item
	}

	for i, val := range data {
		switch i {
		case 0:
			item.ReturnID = val
		case 1:
			item.ExternalOrderID = val
		case 10:
			s := strings.ToLower(val)
			item.Status = ShopeeReturnStatus(s)

		case 11:
			s := strings.ToLower(val)
			item.Type = ShopeeReturnType(s)

		case 13:
			s := strings.ToLower(val)
			item.Solution = Solution(s)
		case 14:
			item.Reason = val
		case 15:
			item.Note = val
		case 20:
			item.Receipt = val

			// case

		}
	}

	return nil
}
