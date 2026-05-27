package command

import (
	_ "myproject/ddd/order/domain"
	_ "myproject/ddd/inventory/event"
	_ "myproject/ddd/inventory/domain" // want "dddcrossdomain"
)
