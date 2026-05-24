package builtin

import "context"

type NopMessageStore struct{}

func NewNopMessageStore() *NopMessageStore { return &NopMessageStore{} }

func (s *NopMessageStore) RecordCommand(context.Context, *CommandEntry) error          { return nil }
func (s *NopMessageStore) RecordQuery(context.Context, *QueryEntry) error              { return nil }
func (s *NopMessageStore) RecordEvent(context.Context, *EventEntry) error              { return nil }
func (s *NopMessageStore) RecordEventHandler(context.Context, *EventHandlerEntry) error { return nil }
