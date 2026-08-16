package shopifysync

import "testing"

func TestSyncRequestValidate(t *testing.T) {
	valid := SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	tests := []struct {
		name   string
		mutate func(*SyncRequest)
	}{
		{name: "organization", mutate: func(request *SyncRequest) { request.OrganizationID = "" }},
		{name: "store", mutate: func(request *SyncRequest) { request.StoreID = "" }},
		{name: "run", mutate: func(request *SyncRequest) { request.RunID = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			test.mutate(&request)
			if err := request.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want missing %s error", test.name)
			}
		})
	}
}

func TestMirrorBatchCountsNestedResources(t *testing.T) {
	batch := MirrorBatch{
		Products:    []Product{{ShopifyGID: "gid://shopify/Product/1"}},
		Variants:    []Variant{{ShopifyGID: "gid://shopify/ProductVariant/1"}},
		Collections: []Collection{{ShopifyGID: "gid://shopify/Collection/1"}},
		Themes:      []Theme{{ShopifyGID: "gid://shopify/OnlineStoreTheme/1"}},
	}
	counts := batch.Counts()
	if counts.Products != 1 || counts.Variants != 1 || counts.Collections != 1 || counts.Themes != 1 {
		t.Fatalf("Counts() = %#v, want one of every resource", counts)
	}
}
