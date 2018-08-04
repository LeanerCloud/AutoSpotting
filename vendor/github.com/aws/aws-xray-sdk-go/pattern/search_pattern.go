// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

// Package pattern provides a basic pattern matching utility.
// Patterns may contain fixed text, and/or special characters (`*`, `?`).
// `*` represents 0 or more wildcard characters. `?` represents a single wildcard character.
package pattern

import "strings"

// WildcardMatchCaseInsensitive returns true if text matches pattern (case-insensitive); returns false otherwise.
func WildcardMatchCaseInsensitive(pattern string, text string) bool {
	return WildcardMatch(pattern, text, true)
}

// WildcardMatch returns true if text matches pattern at the given case-sensitivity; returns false otherwise.
func WildcardMatch(pattern string, text string, caseInsensitive bool) bool {
	patternLen := len(pattern)
	textLen := len(text)
	if 0 == patternLen {
		return 0 == textLen
	}

	if isWildcardGlob(pattern) {
		return true
	}

	if caseInsensitive {
		pattern = strings.ToLower(pattern)
		text = strings.ToLower(text)
	}

	indexOfGlob := strings.Index(pattern, "*")
	if -1 == indexOfGlob || patternLen-1 == indexOfGlob {
		return simpleWildcardMatch(pattern, text)
	}

	res := make([]bool, textLen+1)
	res[0] = true
	for j := 0; j < patternLen; j++ {
		p := pattern[j]
		if '*' != p {
			for i := textLen - 1; i >= 0; i-- {
				t := text[i]
				res[i+1] = res[i] && ('?' == p || t == p)
			}
		} else {
			i := 0
			for i <= textLen && !res[i] {
				i++
			}
			for i <= textLen {
				res[i] = true
				i++
			}
		}
		res[0] = res[0] && '*' == p
	}
	return res[textLen]

}

func simpleWildcardMatch(pattern string, text string) bool {
	j := 0
	patternLen := len(pattern)
	textLen := len(text)
	for i := 0; i < patternLen; i++ {
		p := pattern[i]
		if '*' == p {
			return true
		} else if '?' == p {
			if textLen == j {
				return false
			}
			j++
		} else {
			if j >= textLen {
				return false
			}
			t := text[j]
			if p != t {
				return false
			}
			j++
		}
	}
	return j == textLen
}

func isWildcardGlob(pattern string) bool {
	return pattern == "*"
}
