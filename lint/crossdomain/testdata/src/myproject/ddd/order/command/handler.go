package command

import (
	_ "myproject/ddd/inventory/domain" // want "dddcrossdomain"
	_ "myproject/ddd/inventory/event"
	_ "myproject/ddd/order/domain"
)
