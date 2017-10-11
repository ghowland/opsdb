package opsdb

import (
	"os/user"
	"fmt"
	"strings"
	"gopkg.in/ldap.v2"
	"strconv"
)

type LdapUser struct {
	IsAuthenticated bool
	Error string

	Username string
	Groups []string

	FirstName string
	FullName string
	Email string

	HomeDir string
	Uid int
}

func LdapLogin(username string, password string) LdapUser {
	// Set up return value, we can return any time
	ldap_user := LdapUser{}
	ldap_user.Username = username

	// Get all LDAP auth from config file...  JSON is fine...

	usr, _ := user.Current()
	homedir := usr.HomeDir

	server_port := ReadPathData(fmt.Sprintf("%s/secure/ldap_connect_port.txt", homedir))	// Should contain contents, no newlines: host.domain.com:389
	server_port = strings.Trim(server_port, " \n")

	fmt.Printf("LDAP: %s\n", server_port)

	l, err := ldap.Dial("tcp", server_port)
	if err != nil {
		ldap_user.IsAuthenticated = false
		ldap_user.Error = err.Error()
		return ldap_user
	}
	defer l.Close()

	fmt.Printf("Dial complete\n")

	ldap_password := ReadPathData(fmt.Sprintf("%s/secure/notcleartextpasswords.txt", homedir))	// Should contain exact password, no newlines.
	ldap_password = strings.Trim(ldap_password, " \n")

	sbr := ldap.SimpleBindRequest{}

	ldap_userconnect := ReadPathData(fmt.Sprintf("%s/secure/ldap_userconnectstring.txt", homedir))	// Should contain connection string, no newlines: "dc=example,dc=com"
	ldap_userconnect = strings.Trim(ldap_userconnect, " \n")

	sbr.Username = ldap_userconnect
	sbr.Password = ldap_password
	_, err = l.SimpleBind(&sbr)
	if err != nil {
		ldap_user.IsAuthenticated = false
		ldap_user.Error = err.Error()
		return ldap_user
	}

	fmt.Printf("Bind complete\n")

	// Get User account

	filter := fmt.Sprintf("(uid=%s)", username)
	fmt.Printf("Filter: %s\n", filter)

	//TODO(g): Get these from JSON or something?  Not sure...  Probably JSON.  This is all ghetto, but it keeps things mostly anonymous and flexible
	attributes := []string{"cn", "gidNumber", "givenName", "homeDirectory", "loginShell", "mail", "sn", "uid", "uidNumber", "userPassword"}

	ldap_usersearch := ReadPathData(fmt.Sprintf("%s/secure/ldap_usersearch.txt", homedir))	// Should contain connection string, no newlines: "dc=example,dc=com"
	ldap_usersearch = strings.Trim(ldap_usersearch, " \n")

	sr := ldap.NewSearchRequest(ldap_usersearch, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, filter, attributes, nil)

	user_result, err := l.Search(sr)
	if err != nil {
		ldap_user.IsAuthenticated = false
		ldap_user.Error = err.Error()
		return ldap_user
	}

	fmt.Printf("User Search complete: %d\n", len(user_result.Entries))

	for count, first := range user_result.Entries {

		//username = first.GetAttributeValue("sn")

		fmt.Printf("User %d: %s\n", count, first.DN)

		// Populate the result
		ldap_user.FirstName = first.GetAttributeValue("givenName")
		ldap_user.Email = first.GetAttributeValue("mail")
		ldap_user.FullName = first.GetAttributeValue("cn")
		ldap_user.Uid, _ = strconv.Atoi(first.GetAttributeValue("uidNumber"))


		for _, attr := range attributes {
			fmt.Printf("    %s == %v\n", attr, first.GetAttributeValue(attr))
		}

	}

	// Get group info for User

	filter = "(cn=*)"
	fmt.Printf("Group Filter: %s\n", filter)

	//TODO(g): Get these from JSON or something?  Not sure...  Probably JSON.  This is all ghetto, but it keeps things mostly anonymous and flexible
	attributes = []string{"cn", "gidNumber", "memberUid"}

	ldap_groupsearch := ReadPathData(fmt.Sprintf("%s/secure/ldap_groupsearch.txt", homedir))	// Should contain connection string, no newlines: "ou=groups,dc=example,dc=com"
	ldap_groupsearch = strings.Trim(ldap_groupsearch, " \n")

	sr = ldap.NewSearchRequest(ldap_groupsearch, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, filter, attributes, nil)

	group_result, err := l.Search(sr)
	if err != nil {
		ldap_user.IsAuthenticated = false
		ldap_user.Error = err.Error()
		return ldap_user
	}

	fmt.Printf("Group Search complete: %d\n", len(group_result.Entries))

	user_groups := make([]string, 0)

	for count, first := range group_result.Entries {

		fmt.Printf("Group %d: %s\n", count, first.DN)

		group := first.GetAttributeValue("cn")
		group_users := first.GetAttributeValues("memberUid")

		for _, group_user := range group_users {
			if group_user == username {
				user_groups = append(user_groups, group)
			}
		}
	}

	fmt.Printf("User: %s  Groups: %v\n", username, user_groups)

	// Testing password
	err = l.Bind(fmt.Sprintf("uid=%s,%s", username, ldap_usersearch), password)
	if err != nil {
		ldap_user.IsAuthenticated = false
		ldap_user.Error = err.Error()
		return ldap_user
	}


	fmt.Printf("Password is correct\n")

	//TODO(g): make a struct and pack this data into it:  LdapUser{}
	ldap_user.IsAuthenticated = true
	ldap_user.Groups = user_groups


	return ldap_user
}



func InitUdn() {
	Debug_Udn_Api = true
	Debug_Udn = false

	UdnFunctions = map[string]UdnFunc{
		"__query":        UDN_QueryById,
		"__debug_output": UDN_DebugOutput,
		"__if":           UDN_IfCondition,
		"__end_if":       nil,
		"__else":         UDN_ElseCondition,
		"__end_else":     nil,
		"__else_if":      UDN_ElseIfCondition,
		"__end_else_if":  nil,
		"__not":          UDN_Not,
		"__not_nil":          UDN_NotNil,
		"__iterate":      UDN_Iterate,
		"__end_iterate":  nil,
		"__get":          UDN_Get,
		"__set":          UDN_Set,
		"__get_first":          UDN_GetFirst,		// Takes N strings, which are dotted for udn_data accessing.  The first value that isnt nil is returned.  nil is returned if they all are
		"__get_temp":          UDN_GetTemp,			// Function stack based temp storage
		"__set_temp":          UDN_SetTemp,			// Function stack based temp storage
		"__temp_label":          UDN_GetTempAccessor,		// This takes a string as an arg, like "info", then returns "(__get.'__function_stack.-1.uuid').info".  Later we will make temp data concurrency safe, so when you need accessors as a string, to a temp (like __string_clear), use this
		//"__temp_clear":          UDN_ClearTemp,
		//"__watch": UDN_WatchSyncronization,
		//"___watch_timeout": UDN_WatchTimeout,				//TODO(g): Should this just be an arg to __watch?  I think so...  Like if/else, watch can control the flow...
		//"__end_watch": nil,
		"__test_return":           UDN_TestReturn, // Return some data as a result
		"__test":           UDN_Test,
		"__test_different": UDN_TestDifferent,
		// Migrating from old functions
		//TODO(g): These need to be reviewed, as they are not necessarily the best way to do this, this is just the easiest/fastest way to do this
		"__widget": UDN_Widget,
		// New functions for rendering web pages (finally!)
		//"__template": UDN_StringTemplate,					// Does a __get from the args...
		"__template": UDN_StringTemplateFromValue,					// Does a __get from the args...
		"__template_wrap": UDN_StringTemplateMultiWrap,					// Takes N-2 tuple args, after 0th arg, which is the wrap_key, (also supports a single arg templating, like __template, but not the main purpose).  For each N-Tuple, the new map data gets "value" set by the previous output of the last template, creating a rolling "wrap" function.
		"__template_map": UDN_MapTemplate,		//TODO(g): Like format, for templating.  Takes 3*N args: (key,text,map), any number of times.  Performs template and assigns key into the input map
		"__format": UDN_MapStringFormat,			//TODO(g): Updates a map with keys and string formats.  Uses the map to format the strings.  Takes N args, doing each arg in sequence, for order control
		"__template_short": UDN_StringTemplateFromValueShort,		// Like __template, but uses {{{fieldname}}} instead of {{index .Max "fieldname"}}, using strings.Replace instead of text/template


		//TODO(g): DEPRICATE.  Longer name, same function.
		"__template_string": UDN_StringTemplateFromValue,	// Templates the string passed in as arg_0

		"__string_append": UDN_StringAppend,
		"__string_clear": UDN_StringClear,		// Initialize a string to empty string, so we can append to it again
		"__concat": UDN_StringConcat,
		"__input": UDN_Input,			//TODO(g): This takes any input as the first arg, and then passes it along, so we can type in new input to go down the pipeline...
		"__input_get": UDN_InputGet,			// Gets information from the input, accessing it like __get
		"__function": UDN_StoredFunction,			//TODO(g): This uses the udn_stored_function.name as the first argument, and then uses the current input to pass to the function, returning the final result of the function.		Uses the web_site.udn_stored_function_domain_id to determine the stored function
		"__execute": UDN_Execute,			//TODO(g): Executes ("eval") a UDN string, assumed to be a "Set" type (Target), will use __input as the Source, and the passed in string as the Target UDN

		"__html_encode": UDN_HtmlEncode,		// Encode HTML symbols so they are not taken as literal HTML


		"__array_append": UDN_ArrayAppend,			// Appends the input into the specified target location (args)

		"__array_divide": UDN_ArrayDivide,			//TODO(g): Breaks an array up into a set of arrays, based on a divisor.  Ex: divide=4, a 14 item array will be 4 arrays, of 4/4/4/2 items each.
		"__array_map_remap": UDN_ArrayMapRemap,			//TODO(g): Takes an array of maps, and makes a new array of maps, based on the arg[0] (map) mapping (key_new=key_old)


		"__map_key_delete": UDN_MapKeyDelete,			// Each argument is a key to remove
		"__map_key_set": UDN_MapKeySet,			// Sets N keys, like __format, but with no formatting
		"__map_copy": UDN_MapCopy,			// Make a copy of the current map, in a new map
		"__map_update": UDN_MapUpdate,			// Input map has fields updated with arg0 map

		"__render_data": UDN_RenderDataWidgetInstance,			// Renders a Data Widget Instance:  arg0 = web_data_widget_instance.id, arg1 = widget_instance map update

		"__json_decode": UDN_JsonDecode,			// Decode JSON
		"__json_encode": UDN_JsonEncode,			// Encode JSON

		"__data_get": UDN_DataGet,					// Dataman Get
		"__data_set": UDN_DataSet,					// Dataman Set
		"__data_filter": UDN_DataFilter,			// Dataman Filter

		"__compare_equal": UDN_CompareEqual,		// Compare equality, takes 2 args and compares them.  Returns 1 if true, 0 if false.  For now, avoiding boolean types...
		"__compare_not_equal": UDN_CompareNotEqual,		// Compare equality, takes 2 args and compares them.  Returns 1 if true, 0 if false.  For now, avoiding boolean types...

		"__ddd_render": UDN_DddRender,			// DDD Render.current: the JSON Dialog Form data for this DDD position.  Uses __ddd_get to get the data, and ___ddd_move to change position.

		"__login": UDN_Login,				// Login through LDAP

		//TODO(g): I think I dont need this, as I can pass it to __ddd_render directly
		//"__ddd_move": UDN_DddMove,				// DDD Move position.current.x.y:  Takes X/Y args, attempted to move:  0.1.1 ^ 0.1.0 < 0.1 > 0.1.0 V 0.1.1
		//"__ddd_get": UDN_DddGet,					// DDD Get.current.{}
		//"__ddd_set": UDN_DddSet,					// DDD Set.current.{}
		//"__ddd_delete": UDN_DddDelete,			// DDD Delete.current: Delete the current item (and all it's sub-items).  Append will be used with __ddd_set/move

		//"__increment": UDN_Increment,				// Increment value
		//"__decrement": UDN_Decrement,				// Decrement value
		//"__split": UDN_StringSplit,				// Split a string into an array on a separator string
		//"__join": UDN_StringJoin,					// Join an array into a string on a separator string
		//"__render_page": UDN_RenderPage,			// Render a page, and return it's widgets so they can be dynamically updated

		// New

		//"__array_append": UDN_ArrayAppend,			//TODO(g): Appends a element onto an array.  This can be used to stage static content so its not HUGE on one line too...

		//"__map_update_prefix": UDN_MapUpdatePrefix,			//TODO(g): Merge a the specified map into the input map, with a prefix, so we can do things like push the schema into the row map, giving us access to the field names and such
		//"__map_clear": UDN_MapClear,			//TODO(g): Clears everything in a map "bucket", like: __map_clear.'temp'

		//"__function_domain": UDN_StoredFunctionDomain,			//TODO(g): Just like function, but allows specifying the udn_stored_function_domain.id as well, so we can use different namespaces.
		//"__capitalize": UDN_StringCapitalize,			//TODO(g): This capitalizes words, title-style
		//"__pluralize": UDN_StringPluralize,			//TODO(g): This pluralizes words, or tries to at least
		//"__starts_with": UDN_StringStartsWith,			//TODO(g): Returns bool if a string starts with the specified arg[0] string
		//"__ends_with": UDN_StringEndsWith,			//TODO(g): Returns bool if a string starts with the specified arg[0] string
		//"__split": UDN_StringSplit,			//TODO(g): Split a string on a value, with a maximum number of splits, and the slice of which to use, with a join as optional value.   The args go:  0) separate to split on,  2)  maximum number of times to split (0=no limit), 3) location to write the split out data (ex: `temp.action.fieldname`) , 3) index of the split to pull out (ex: -1, 0, 1, for the last, first or second)  4) optional: the end of the index to split on, which creates an array of items  5) optional: the join value to join multiple splits on (ex: `_`, which would create a string like:  `second_third_forth` out of a [1,4] slice)
		//"__get_session_data": UDN_SessionDataGet,			//TODO(g): Get something from a safe space in session data (cannot conflict with internal data)
		//"__set_session_data": UDN_SessionDataGet,			//TODO(g): Set something from a safe space in session data (cannot conflict with internal data)
		//"__continue": UDN_IterateContinue,		// Skip to next iteration
		// -- Dont think I need this -- //"__break": UDN_IterateBreak,				//TODO(g): Break this iteration, we are done.  Is this needed?  Im not sure its needed, and it might suck

		// Allows safe concurrency operations...
		//"__set_temp": UDN_Set_Temp,		// Sets a temporary variable.  Is safe for this sequence, and cannot conflict with our UDN setting the same names as temp vars in other threads
		//"__get_temp": UDN_Set_Temp,		// Gets a temporary variable.  Is safe for this sequence, and cannot conflict with our UDN setting the same names as temp vars in other threads
	}

	PartTypeName = map[int]string{
		int(part_unknown): "Unknown",
		int(part_function): "Function",
		int(part_item): "Item",
		int(part_string): "String",
		int(part_compound): "Compound",
		int(part_list): "List",
		int(part_map): "Map",
		int(part_map_key): "Map Key",
	}
}


