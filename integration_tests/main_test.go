// Copyright 2021 TiKV Authors
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

package tikv_test

import (
	"testing"

	"github.com/ergesun/client-go/v2/tikv"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	tikv.EnableFailpoints()

	opts := []goleak.Option{
		goleak.IgnoreTopFunction("github.com/pingcap/goleveldb/leveldb.(*DB).mpoolDrain"),
		goleak.IgnoreTopFunction("github.com/ergesun/client-go/v2/txnkv/transaction.keepAlive"), // TODO: fix ttlManager goroutine leak
	}

	goleak.VerifyTestMain(m, opts...)
}
