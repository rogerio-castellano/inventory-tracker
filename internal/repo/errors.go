package repo

import "errors"

var ErrDuplicatedValueUnique = errors.New("could not create record: unique field value duplicated")
