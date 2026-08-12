// Copyright 2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import "errors"

var (
	// ErrFlagsMutuallyExclusive indicates that both --header and --header-file
	// flags were provided, but only one is allowed at a time.
	ErrFlagsMutuallyExclusive = errors.New("--header and --header-file are mutually exclusive")

	// ErrHeaderFilePathInvalid indicates that the path supplied to the
	// --header-file flag is not a well-formed file path.
	ErrHeaderFilePathInvalid = errors.New("header file path is invalid")

	// ErrHeaderFileNotFound indicates that the file referenced by the
	// --header-file flag does not exist on disk.
	ErrHeaderFileNotFound = errors.New("header file not found")

	// ErrHeaderFileNotReadable indicates that the file referenced by the
	// --header-file flag exists but could not be read due to permission
	// or I/O errors.
	ErrHeaderFileNotReadable = errors.New("header file could not be read")

	// ErrHeaderTooLarge indicates that the resolved comment text exceeds the
	// maximum allowed size of 8,192 bytes.
	ErrHeaderTooLarge = errors.New("header comment exceeds maximum size")

	// ErrConfigSyntax indicates that a configuration file contains invalid
	// syntax that prevents parsing.
	ErrConfigSyntax = errors.New("invalid config file")

	// ErrConfigNotReadable indicates that a configuration file exists but could
	// not be read due to permission or I/O errors.
	ErrConfigNotReadable = errors.New("config file not readable")

	// ErrIgnorePatternInvalid indicates that an ignore pattern in the
	// configuration file is malformed and cannot be used for directory
	// matching.
	ErrIgnorePatternInvalid = errors.New("invalid ignore pattern in config")
)
