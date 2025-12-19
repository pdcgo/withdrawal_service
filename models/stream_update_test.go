package models_test

import (
	"testing"

	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/pdcgo/withdrawal_service/models"
	"github.com/stretchr/testify/assert"
)

func TestParseModelWditem(t *testing.T) {
	t.Run("testing normal", func(t *testing.T) {
		data := []string{
			"2024-10-01 02:28",
			"Saldo Penjual",
			"Penghasilan dari Pesanan #2409260H0PHYX6",
			"2409260H0PHYX6",
			"Transaksi Masuk",
			"106663.00",
			"Transaksi Selesai",
			"1520373.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)
	})

	t.Run("testing data tanggal", func(t *testing.T) {
		data := []string{
			"2024-10-01 02:28:12",
			"Saldo Penjual",
			"Penghasilan dari Pesanan #2409260H0PHYX6",
			"2409260H0PHYX6",
			"Transaksi Masuk",
			"106663.00",
			"Transaksi Selesai",
			"1413710.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)

		assert.Equal(t, "2024-10-01 02:28:12", item.TransactionDate.Format("2006-01-02 15:04:05"))
	})

	t.Run("testing data kurang", func(t *testing.T) {
		data := []string{
			"2024-10-01 02:28",
			"Saldo Penjual",
			"Penghasilan dari Pesanan #2409260H0PHYX6",
			"2409260H0PHYX6",
			"Transaksi Masuk",
			"106663.00",
			"Transaksi Selesai",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.NotNil(t, err)
	})

	t.Run("testing data kompensasi", func(t *testing.T) {
		data := []string{
			"2024-12-01 09:54:05",
			"Penyesuaian",
			"Auto-approve compensation without judging",
			"24112437GWJ229",
			"Transaksi Masuk",
			"119001.00",
			"Transaksi Selesai",
			"7917635.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)

		assert.True(t, item.IsCompensationAdjustment())
		_, offset := item.TransactionDate.Zone()
		assert.Equal(t, 7, offset/3600)
	})

	t.Run("testing penyesuaian", func(t *testing.T) {
		data := []string{
			"2024-11-18 11:13:32",
			"Penyesuaian",
			"Penyesuaian untuk 241106G3CQC53J",
			"241106G3CQC53J",
			"Transaksi Masuk",
			"5000.00",
			"Transaksi Selesai",
			"10950271.00",
		}

		item := models.ShopeeWdItem{}
		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)
	})

	t.Run("testing data kompensasi kehilangan", func(t *testing.T) {

		data := []string{
			"2024-11-21 16:59:49",
			"Penyesuaian",
			"Kompensasi kehilangan untuk paket #SPXID04888664784B pada pesanan #241110T91D5KB2. Untuk Penjual yang mengaktifkan Asuransi Pengiriman Shopee, pengajuan klaim sedang diproses. Mohon ditunggu",
			"241110T91D5KB2",
			"Transaksi Masuk",
			"129800.00",
			"Transaksi Selesai",
			"610887.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)

		assert.True(t, item.IsLostCompensationAdjustment())

	})

	t.Run("testing 350 biaya admin", func(t *testing.T) {
		data := []string{
			"2024-11-28 23:20:24",
			"Penghasilan dari Pesanan",
			"Penghasilan dari Pesanan #241117G5TWV5SS",
			"241117G5TWV5SS",
			"Transaksi Keluar",
			"-350.00",
			"Transaksi Selesai",
			"8722351.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)
	})

	t.Run("testing orderid biaya premi", func(t *testing.T) {
		data := []string{
			"2024-12-11 09:21:12",
			"Penyesuaian",
			"Penyesuaian Saldo Penjual untuk biaya premi Pesanan yang Gagal Terkirim: 2411122Y5PW79T",
			"-",
			"Transaksi Keluar",
			"-618.00",
			"Transaksi Selesai",
			"245377.00",
		}

		item := models.ShopeeWdItem{}

		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)

		assert.Equal(t, "2411122Y5PW79T", item.OrderIDExtractFromDesc())
	})

}

func TestModelShopeeOrderCancel(t *testing.T) {
	t.Run("testing normal", func(t *testing.T) {
		data := []string{
			"24092989VWP90G",
			"Batal",
			"Dibatalkan oleh Pembeli. Alasan: Perlu mengubah alamat pengiriman",
			"",
			"",
			"Hemat",
			"",
			"",
			"",
			"2024-09-29 12:05",
			"-",
			"Online Payment",
			"",
			"OOTD One Set Perempuan (Vest Rajut  + Kemeja + Celana Cargo) Outfit Korean Style - KK123",
			"",
			"vest celana",
			"149.650",
			"149.650",
			"1",
			"0",
			"149.650",
			"0",
			"0",
			"0",
			"800 gr",
			"1",
			"800 gr",
			"0",
			"0",
			"0",
			"N",
			"0",
			"0",
			"0",
			"0",
			"0",
			"0",
			"0",
			"0",
			"52.500",
			"",
			"",
			"bx4asnvcdy",
			"D***a",
			"******08",
			"Ja******",
			"KAB. KOLAKA UTARA",
			"SULAWESI TENGGARA",
			"",
		}

		item := models.ShopeeOrderCancel{}

		err := item.Marshal(data)
		assert.Nil(t, err)
	})
}

func TestModelShopeeReturn(t *testing.T) {
	data := []string{"2409140W37NQ1W4", "240911MA9FAHR6", "2024-09-11 03:10", "y******1", "Skena Outfit One Set Wanita (Kemeja Flannel, tanktop, Celana Cargo) Oneset OOTD Korean Style - KK08", "13-WUL-B-X-3F", "kemeja", "", "RP 80.458\n", "2024-09-14 14:58", "Banding Ditolak", "Seluruh Pesanan", "1", "Pengembalian Barang dan Dana", "Produk yang diterima berbeda dengan deskripsi.", "", "RP 80.458\n", "2024-09-15 19:40", "Tidak", "SPX Express", "ID246791218694Y", "Pengiriman pengembalian barang selesai", "2024-09-16 17:21", "hj Umar Sumberagung pecaton RT.18 rw 02 路 Sumberagung, Jawa Timur, Kab. Malang, Sumbermanjing Wetan , Sumbermanjing Wetan , Kab. Malang , Jawa Timur Kab. Malang, KAB. MALANG, SUMBERMANJING WETAN, JAWA TIMUR, ID, 65176", "JAWA TIMUR", "KAB. MALANG", "65176", "6285928068858", "", "", "", "", "", "", "Tidak menerima pengembalian barang", "produk belum kembali ke tempat saya ya kak mohon dibantu follow up kurir yaa", "-", "Reguler", "Fulfilled by Seller", "RP 105.004\n", "COD", ""}

	item := models.ShopeeReturnOrder{}
	err := item.Marshal(data)

	// da, _ := json.MarshalIndent(item, "", "  ")

	// t.Error(string(da))
	assert.Nil(t, err)
}
