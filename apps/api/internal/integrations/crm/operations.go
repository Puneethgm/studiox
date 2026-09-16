package crm

// OperationKey is the fixed vocabulary every onboarded CRM gets mapped onto.
// This list mirrors what internal/integrations/glofox.Client already
// implements today — these are the only things the platform actually needs
// a CRM for. Adding a new operation means adding a case here (a code
// change); mapping an *existing* operation onto a *new* CRM never does.
type OperationKey string

const (
	OpCreateLead         OperationKey = "create_lead"
	OpRegisterUser       OperationKey = "register_user"
	OpPurchaseMembership OperationKey = "purchase_membership"
	OpFindMembershipPlan OperationKey = "find_membership_plan"
	OpGetMember          OperationKey = "get_member"
	OpListBookings       OperationKey = "list_bookings"
)

// OperationSpec describes one catalog entry for the doc-parsing prompt and
// for validating an admin-edited mapping.
type OperationSpec struct {
	Key         OperationKey
	Description string
	Params      []string // names the executor will substitute into path/query/body
}

// Catalog is the ordered list of operations shown in the review UI and sent
// to the LLM when parsing an uploaded CRM doc.
var Catalog = []OperationSpec{
	{
		Key:         OpCreateLead,
		Description: "Register a new prospect (not yet a paying member) in the CRM.",
		// birthDate/street/city/state/postalCode/referredBy are optional —
		// only present because some CRMs (e.g. Mindbody) require a full
		// profile to accept a new client and we don't collect that much on
		// a lead. Callers fill them with neutral placeholders when unused.
		Params: []string{"email", "firstName", "lastName", "phone", "leadStatus",
			"birthDate", "street", "city", "state", "postalCode", "referredBy"},
	},
	{
		Key:         OpRegisterUser,
		Description: "Create a full member/user record with login credentials.",
		Params:      []string{"email", "firstName", "lastName", "phone", "password"},
	},
	{
		Key:         OpPurchaseMembership,
		Description: "Charge an existing member for a trial or membership plan.",
		Params:      []string{"userId", "membershipId", "planCode", "startDate"},
	},
	{
		Key:         OpFindMembershipPlan,
		Description: "Look up a membership/plan by price, to auto-detect which plan a payment corresponds to.",
		Params:      []string{"amountCents", "isTrial"},
	},
	{
		Key:         OpGetMember,
		Description: "Fetch one member's current profile/status by their CRM user id.",
		Params:      []string{"userId"},
	},
	{
		Key:         OpListBookings,
		Description: "Read a branch/location's class or session bookings.",
		Params:      []string{},
	},
}

// Valid reports whether k is one of the fixed catalog keys.
func (k OperationKey) Valid() bool {
	for _, s := range Catalog {
		if s.Key == k {
			return true
		}
	}
	return false
}
