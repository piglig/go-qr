// Package detect holds the image analysis of package qr's decoder: the
// luminance of an image and its pyramid of halvings, the locally adaptive
// threshold, finder pattern detection, fitting the module grid of a symbol
// to its structure, and reading module values and their reliabilities.
// Package qr searches with these and decodes what they sample.
package detect
