// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"bufio"
	"os"
	"strings"
)

// EnvironmentPrefix defines a string that will be implicitely prefixed to a
// flag name before looking it up in the environment variables.
var EnvironmentPrefix = ""

// ParseEnv parses flags from environment variables.
// Flags already set will be ignored.
func (f *FlagSet) ParseEnv(environ []string) error {

	m := f.formal

	env := make(map[string]string)
	for _, s := range environ {
		i := strings.Index(s, "=")
		if i < 1 {
			continue
		}
		env[s[0:i]] = s[i+1 : len(s)]
	}

	for _, flag := range m {
		name := flag.Name
		_, set := f.actual[name]
		if set {
			continue
		}

		flag, alreadythere := m[name]
		if !alreadythere {
			if name == "help" || name == "h" { // special case for nice help message.
				f.usage()
				return ErrHelp
			}
			return f.failf("environment variable provided but not defined: %s", name)
		}

		envKey := strings.ToUpper(flag.Name)
		if f.envPrefix != "" {
			envKey = f.envPrefix + "_" + envKey
		}
		envKey = strings.Replace(envKey, "-", "_", -1)

		value, isSet := env[envKey]
		if !isSet {
			continue
		}

		hasValue := false
		if len(value) > 0 {
			hasValue = true
		}

		if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
			if hasValue {
				if err := fv.Set(value); err != nil {
					return f.failf("invalid boolean value %q for environment variable %s: %v", value, name, err)
				}
			} else {
				// flag without value is regarded a bool
				fv.Set("true")
			}
		} else {
			if err := flag.Value.Set(value); err != nil {
				return f.failf("invalid value %q for environment variable %s: %v", value, name, err)
			}
		}

		// update f.actual
		if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[name] = flag

	}
	return nil
}

// NewFlagSetWithEnvPrefix returns a new empty flag set with the specified name,
// environment variable prefix, and error handling property.
func NewFlagSetWithEnvPrefix(name string, prefix string, errorHandling ErrorHandling) *FlagSet {
	f := NewFlagSet(name, errorHandling)
	f.envPrefix = prefix
	return f
}

// DefaultConfigFlagname defines the flag name of the optional config file
// path. Used to lookup and parse the config file when a default is set and
// available on disk.
var DefaultConfigFlagname = "config"

// ParseFile parses flags from the file in path.
// Same format as commandline argumens, newlines and lines beginning with a
// "#" charater are ignored. Flags already set will be ignored.
func (f *FlagSet) ParseFile(path string) error {

	// Extract arguments from file
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()

		// Ignore empty lines
		if len(line) == 0 {
			continue
		}

		// Ignore comments
		if line[:1] == "#" {
			continue
		}

		// Match `key=value` and `key value`
		var name, value string
		hasValue := false
		for i, v := range line {
			if v == '=' || v == ' ' {
				hasValue = true
				name, value = line[:i], line[i+1:]
				break
			}
		}

		if hasValue == false {
			name = line
		}

		// Ignore flag when already set; arguments have precedence over file
		if f.actual[name] != nil {
			continue
		}

		m := f.formal
		flag, alreadythere := m[name]
		if !alreadythere {
			if name == "help" || name == "h" { // special case for nice help message.
				f.usage()
				return ErrHelp
			}
			return f.failf("configuration variable provided but not defined: %s", name)
		}

		if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
			if hasValue {
				if err := fv.Set(value); err != nil {
					return f.failf("invalid boolean value %q for configuration variable %s: %v", value, name, err)
				}
			} else {
				// flag without value is regarded a bool
				fv.Set("true")
			}
		} else {
			if err := flag.Value.Set(value); err != nil {
				return f.failf("invalid value %q for configuration variable %s: %v", value, name, err)
			}
		}

		// update f.actual
		if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[name] = flag
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
