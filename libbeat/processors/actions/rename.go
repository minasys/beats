package actions

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/processors"
)

type renameFields struct {
	config renameFieldsConfig
}

type renameFieldsConfig struct {
	Fields        []fromTo `config:"fields"`
	IgnoreMissing bool     `config:"ignore_missing"`
	FailOnError   bool     `config:"fail_on_error"`
}

type fromTo struct {
	From string `config:"from"`
	To   string `config:"to"`
}

func init() {
	processors.RegisterPlugin("rename",
		configChecked(newRenameFields,
			requireFields("fields")))
}

func newRenameFields(c *common.Config) (processors.Processor, error) {

	cfgwarn.Beta("Beta rename processor is used.")
	config := renameFieldsConfig{
		IgnoreMissing: false,
		FailOnError:   true,
	}
	err := c.Unpack(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack the rename configuration: %s", err)
	}

	f := &renameFields{
		config: config,
	}
	return f, nil
}

func (f *renameFields) Run(event *beat.Event) (*beat.Event, error) {
	var backup common.MapStr
	// Creates a copy of the event to revert in case of failure
	if f.config.FailOnError {
		backup = event.Fields.Clone()
	}

	for _, field := range f.config.Fields {
		err := f.renameField(field.From, field.To, event.Fields)
		if err != nil && f.config.FailOnError {
			logp.Debug("rename", "Failed to rename fields, revert to old event: %s", err)
			event.Fields = backup
			return event, err
		}
	}

	return event, nil
}

func (f *renameFields) renameField(from string, to string, fields common.MapStr) error {
	// Fields cannot be overwritten. Either the target field has to be dropped first or renamed first
	exists, _ := fields.HasKey(to)
	if exists {
		return fmt.Errorf("target field %s already exists, drop or rename this field first", to)
	}

	value, err := fields.GetValue(from)
	if err != nil {
		// Ignore ErrKeyNotFound errors
		if f.config.IgnoreMissing && errors.Cause(err) == common.ErrKeyNotFound {
			return nil
		}
		return fmt.Errorf("could not fetch value for key: %s, Error: %s", to, err)
	}

	// Deletion must happen first to support cases where a becomes a.b
	err = fields.Delete(from)
	if err != nil {
		return fmt.Errorf("could not delete key: %s,  %+v", from, err)
	}

	_, err = fields.Put(to, value)
	if err != nil {
		return fmt.Errorf("could not put value: %s: %v, %+v", to, value, err)
	}
	return nil
}

func (f *renameFields) String() string {
	return "rename=" + fmt.Sprintf("%+v", f.config.Fields)
}
