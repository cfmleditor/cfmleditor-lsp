component {

    // --- Component references via different patterns ---

    // Definition: new keyword (dotted path)
    variables.widget = new models.Widget("test");

    // Definition: createObject("component", "path")
    variables.user = createObject("component", "models.User");

    // Definition: createObject with single arg (default type)
    variables.base = createObject("models.Base");

    // Definition: component resolver (getService pattern)
    variables.userService = getService("UserService");
    variables.widgetService = getService("WidgetService");

    // Definition: variable name resolver (_parent)
    variables._parent = "";

    function init() {
        // Definition: new keyword inside function
        var localWidget = new models.Widget("local", "green");

        // Definition: createObject inside function
        var localUser = createObject("component", "models.User");

        return this;
    }

    function testMethodCalls() {
        // Definition: method on component variable (dot-qualified)
        var html = variables.widget.render();
        var name = variables.widget.getName();

        // Definition: method via resolver-assigned variable
        var user = variables.userService.getUser(1);
        var newUser = variables.userService.createUser(1, "bob", "bob@test.com");

        // Definition: method on _parent (resolver)
        var className = variables._parent.getClassName();

        // Definition: unqualified function call (same component)
        var helper = helperMethod();

        return html;
    }

    function helperMethod() {
        return "helper";
    }

    // Definition: method on new expression chain
    function testChainedNew() {
        var name = new models.Widget("x").getName();
        var cls = new models.Base().getClassName();
    }

    // Definition: method on createObject chain
    function testChainedCreateObject() {
        var name = createObject("component", "models.Widget").getName();
    }

    // The ORM entity factories resolve the way createObject does, and are here
    // so internal/tsoracle can record them as deliberate differences rather
    // than as gaps — the grammar reads them as calls. They sit at the end of
    // the file because definition_testdata_test.go addresses this fixture by
    // line number, so anything inserted above moves every one of its cases.
    function testEntityFactories() {
        var made = entityNew("models.User");
        var found = entityLoad("models.User");
    }

}
