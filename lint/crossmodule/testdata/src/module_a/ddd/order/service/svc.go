package service

import (
	_ "module_b/ddd/inventory/domain"    // want "dddcrossmodule"
	_ "module_b/ddd/inventory/event"
	_ "module_a/ddd/order/domain"
)
