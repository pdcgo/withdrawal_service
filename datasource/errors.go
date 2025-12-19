package datasource

import "errors"

var ErrContainInProcessWD = errors.New("ada withdrawal yang masih diproses, silahkan reimport kembali ketika withdrawalnya menjadi selesai")
var ErrCannotGetMarketplaceUsername = errors.New("cannot get marketplace username")
