/*
 * Copyright (C) 2022 Nuts community
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

// Package didservice contains DID Document related functionality that only matters to the current node.
// All functionality here has zero relations to the network.
package didservice

import (
	"crypto"
	"errors"
	"fmt"
	"github.com/nuts-foundation/nuts-node/crypto/hash"
	"github.com/nuts-foundation/nuts-node/vdr/didstore"
	"time"

	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-did/did"
	"github.com/nuts-foundation/nuts-node/vdr/types"
)

// ErrNestedDocumentsTooDeep is returned when a DID Document contains a multiple services with the same type
var ErrNestedDocumentsTooDeep = errors.New("DID Document controller structure has too many indirections")

// DefaultMaxServiceReferenceDepth holds the default max. allowed depth for DID service references.
const DefaultMaxServiceReferenceDepth = 5

const maxControllerDepth = 5

// Resolver implements the DIDResolver interface with a types.Store as backend
type Resolver struct {
	Store didstore.Store
}

func (d Resolver) Resolve(id did.DID, metadata *types.ResolveMetadata) (*did.Document, *types.DocumentMetadata, error) {
	return resolve(d.Store, id, metadata, 0)
}

func resolve(resolver types.DIDResolver, id did.DID, metadata *types.ResolveMetadata, depth int) (*did.Document, *types.DocumentMetadata, error) {
	if depth >= maxControllerDepth {
		return nil, nil, ErrNestedDocumentsTooDeep
	}

	doc, meta, err := resolver.Resolve(id, metadata)
	if err != nil {
		return nil, nil, err
	}

	// has the doc controllers, should we check for controller deactivation?
	if len(doc.Controller) > 0 && (metadata == nil || !metadata.AllowDeactivated) {
		// also check if the controller is not deactivated
		// since ResolveControllers calls Resolve and propagates the metadata
		controllers, err := resolveControllers(resolver, *doc, metadata, depth+1)
		if err != nil {
			return nil, nil, err
		}
		// doc should have controllers, but no results, so they are not active, return error:
		if len(controllers) == 0 {
			return nil, nil, types.ErrNoActiveController
		}
	}

	return doc, meta, nil
}

// ResolveControllers finds the DID Document controllers
func ResolveControllers(resolver types.DIDResolver, doc did.Document, metadata *types.ResolveMetadata) ([]did.Document, error) {
	return resolveControllers(resolver, doc, metadata, 0)
}

func resolveControllers(resolver types.DIDResolver, doc did.Document, metadata *types.ResolveMetadata, depth int) ([]did.Document, error) {
	var leaves []did.Document
	var refsToResolve []did.DID

	if len(doc.Controller) == 0 && len(doc.CapabilityInvocation) > 0 {
		// no controller -> doc is its own controller
		leaves = append(leaves, doc)
	} else {
		for _, ctrlDID := range doc.Controller {
			if doc.ID.Equals(ctrlDID) {
				if len(doc.CapabilityInvocation) > 0 {
					// doc is its own controller
					leaves = append(leaves, doc)
				}
			} else {
				// add did to be resolved later
				refsToResolve = append(refsToResolve, ctrlDID)
			}
		}
	}

	// resolve all unresolved doc
	for _, ref := range refsToResolve {
		node, _, err := resolve(resolver, ref, metadata, depth)
		if errors.Is(err, types.ErrDeactivated) || errors.Is(err, types.ErrNoActiveController) {
			continue
		}
		if errors.Is(err, ErrNestedDocumentsTooDeep) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("unable to resolve controller ref: %w", err)
		}
		leaves = append(leaves, *node)
	}

	// filter deactivated
	j := 0
	for _, leaf := range leaves {
		if !IsDeactivated(leaf) {
			leaves[j] = leaf
			j++
		}
	}

	return leaves[:j], nil
}

// NutsKeyResolver implements the NutsKeyResolver interface.
type NutsKeyResolver struct {
	Resolver types.DIDResolver
}

func (r NutsKeyResolver) ResolvePublicKey(kid string, sourceTransactionsRefs []hash.SHA256Hash) (crypto.PublicKey, error) {
	// try all keys, continue when err == types.ErrNotFound
	for _, h := range sourceTransactionsRefs {
		publicKey, err := r.resolvePublicKey(r.Resolver, kid, types.ResolveMetadata{
			SourceTransaction: &h,
		})
		if err == nil {
			return publicKey, nil
		}
		if err != types.ErrNotFound {
			return nil, err
		}
	}

	return nil, types.ErrNotFound
}

// KeyResolver implements the KeyResolver interface with a types.Store as backend
type KeyResolver struct {
	Store didstore.Store
}

// ResolveSigningKeyID resolves the ID of the first valid AssertionMethod for an indicated DID document at a given time.
func (r KeyResolver) ResolveSigningKeyID(holder did.DID, validAt *time.Time) (string, error) {
	doc, _, err := r.Store.Resolve(holder, &types.ResolveMetadata{
		ResolveTime: validAt,
	})
	if err != nil {
		return "", err
	}
	if len(doc.AssertionMethod) == 0 {
		return "", types.ErrKeyNotFound
	}
	return doc.AssertionMethod[0].ID.String(), nil
}

// ResolveSigningKey resolves the PublicKey of the first valid AssertionMethod for an indicated
// DID document at a validAt time.
func (r KeyResolver) ResolveSigningKey(keyID string, validAt *time.Time) (crypto.PublicKey, error) {
	return r.ResolveRelationKey(keyID, validAt, types.AssertionMethod)
}

func (r KeyResolver) ResolveRelationKey(keyID string, validAt *time.Time, relationType types.RelationType) (crypto.PublicKey, error) {
	holder, err := GetDIDFromURL(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID (id=%s): %w", keyID, err)
	}
	doc, _, err := r.Store.Resolve(holder, &types.ResolveMetadata{
		ResolveTime: validAt,
	})
	if err != nil {
		return "", err
	}
	relationships, _ := resolveRelationships(doc, relationType)

	for _, rel := range relationships {
		if rel.ID.String() == keyID {
			return rel.PublicKey()
		}
	}
	return "", types.ErrKeyNotFound
}

func resolveRelationships(doc *did.Document, relationType types.RelationType) (relationships did.VerificationRelationships, err error) {
	switch relationType {
	case types.Authentication:
		return doc.Authentication, nil
	case types.AssertionMethod:
		return doc.AssertionMethod, nil
	case types.KeyAgreement:
		return doc.KeyAgreement, nil
	case types.CapabilityInvocation:
		return doc.CapabilityInvocation, nil
	case types.CapabilityDelegation:
		return doc.CapabilityDelegation, nil
	default:
		return nil, fmt.Errorf("unable to locate RelationType %v", relationType)
	}
}

// ResolveAssertionKeyID resolves the id of the first valid AssertionMethod of an indicated DID document in the current state.
func (r KeyResolver) ResolveAssertionKeyID(id did.DID) (ssi.URI, error) {
	doc, _, err := r.Store.Resolve(id, nil)
	if err != nil {
		return ssi.URI{}, err
	}

	return ExtractFirstRelationKeyIDByType(*doc, types.AssertionMethod)
}

func (r KeyResolver) ResolveRelationKeyID(id did.DID, relationType types.RelationType) (ssi.URI, error) {
	doc, _, err := r.Store.Resolve(id, nil)
	if err != nil {
		return ssi.URI{}, err
	}

	return ExtractFirstRelationKeyIDByType(*doc, relationType)
}

// ResolveKeyAgreementKey resolves the public key of the first valid KeyAgreement of an indicated DID document in the current state.
// If the document has no KeyAgreements, types.ErrKeyNotFound is returned.
func (r KeyResolver) ResolveKeyAgreementKey(id did.DID) (crypto.PublicKey, error) {
	doc, _, err := r.Store.Resolve(id, nil)
	if err != nil {
		return ssi.URI{}, err
	}
	if len(doc.KeyAgreement) == 0 {
		return nil, types.ErrKeyNotFound
	}
	return doc.KeyAgreement[0].PublicKey()
}

// ExtractFirstRelationKeyIDByType returns the first relation key ID from the given DID document matching the relationType.
// Returns a types.ErrKeyNotFound if no relation key of the given relationType is present.
func ExtractFirstRelationKeyIDByType(doc did.Document, relationType types.RelationType) (ssi.URI, error) {
	keys, err := resolveRelationships(&doc, relationType)
	if err != nil {
		return ssi.URI{}, err
	}
	for _, key := range keys {
		kid := key.ID.String()
		u, _ := ssi.ParseURI(kid)
		return *u, nil
	}

	return ssi.URI{}, types.ErrKeyNotFound
}

func (r NutsKeyResolver) resolvePublicKey(resolver types.DIDResolver, kid string, metadata types.ResolveMetadata) (crypto.PublicKey, error) {
	id, err := did.ParseDIDURL(kid)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID (id=%s): %w", kid, err)
	}
	holder, _ := GetDIDFromURL(kid) // can't fail, already parsed
	doc, _, err := resolver.Resolve(holder, &metadata)
	if err != nil {
		return nil, err
	}

	vm := doc.VerificationMethod.FindByID(*id)
	if vm == nil {
		return nil, types.ErrKeyNotFound
	}

	return vm.PublicKey()
}

// ServiceResolver is a wrapper around a DID store that allows resolving services, following references.
type ServiceResolver struct {
	Store didstore.Store
}

func (s ServiceResolver) Resolve(query ssi.URI, maxDepth int) (did.Service, error) {
	return s.ResolveEx(query, 0, maxDepth, map[string]*did.Document{})
}

func (s ServiceResolver) ResolveEx(endpoint ssi.URI, depth int, maxDepth int, documentCache map[string]*did.Document) (did.Service, error) {
	if depth >= maxDepth {
		return did.Service{}, types.ErrServiceReferenceToDeep
	}

	referencedDID, err := GetDIDFromURL(endpoint.String())
	if err != nil {
		// Shouldn't happen, because only DID URLs are passed?
		return did.Service{}, err
	}
	var document *did.Document
	if document = documentCache[referencedDID.String()]; document == nil {
		document, _, err = Resolver{Store: s.Store}.Resolve(referencedDID, nil)
		if err != nil {
			return did.Service{}, err
		}
		documentCache[referencedDID.String()] = document
	}

	var service *did.Service
	for _, curr := range document.Service {
		if curr.Type == endpoint.Query().Get(serviceTypeQueryParameter) {
			// If there are multiple services with the same type the document is conflicted.
			// This can happen temporarily during a service update (delete old, add new).
			// Both endpoints are likely to be active in the timeframe that the conflict exists, so picking the first entry is preferred for availability over an error.
			service = &curr
			break
		}
	}
	if service == nil {
		return did.Service{}, types.ErrServiceNotFound
	}

	var endpointURL string
	if service.UnmarshalServiceEndpoint(&endpointURL) == nil {
		// Service endpoint is a string, if it's a reference we need to resolve it
		if IsServiceReference(endpointURL) {
			// Looks like a reference, recurse
			resolvedEndpointURI, err := ssi.ParseURI(endpointURL)
			if err != nil {
				return did.Service{}, err
			}
			err = ValidateServiceReference(*resolvedEndpointURI)
			if err != nil {
				return did.Service{}, err
			}
			return s.ResolveEx(*resolvedEndpointURI, depth+1, maxDepth, documentCache)
		}
	}
	return *service, nil
}

// IsFunctionalResolveError returns true if the given error indicates the DID or service not being found or invalid,
// e.g. because it is deactivated, referenced too deeply, etc.
func IsFunctionalResolveError(target error) bool {
	return errors.Is(target, types.ErrNotFound) ||
		errors.Is(target, types.ErrDeactivated) ||
		errors.Is(target, types.ErrServiceNotFound) ||
		errors.Is(target, types.ErrNoActiveController) ||
		errors.Is(target, types.ErrServiceReferenceToDeep) ||
		errors.Is(target, did.InvalidDIDErr) ||
		errors.As(target, new(DIDServiceQueryError))
}
