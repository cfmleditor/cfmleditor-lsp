<cfcomponent>

    <!--- Definition: createObject in cfset --->
    <cfset variables.widget = createObject("component", "models.Widget")>

    <!--- Definition: new keyword in cfset --->
    <cfset variables.user = new models.User(1, "test", "test@test.com")>

    <!--- Definition: cfobject tag --->
    <cfobject component="models.Base" name="variables.base">

    <!--- Definition: component resolver --->
    <cfset variables.userService = getService("UserService")>

    <cffunction name="init" access="public" returntype="DefinitionTestTag">
        <cfreturn this>
    </cffunction>

    <cffunction name="testMethodCalls" access="public" returntype="string">
        <!--- Definition: method on component variable --->
        <cfset var html = variables.widget.render()>
        <cfset var name = variables.widget.getName()>

        <!--- Definition: method via resolver --->
        <cfset var user = variables.userService.getUser()>

        <!--- Definition: unqualified function call --->
        <cfset var helper = helperMethod()>

        <cfreturn html>
    </cffunction>

    <cffunction name="helperMethod" access="private" returntype="string">
        <cfreturn "helper">
    </cffunction>

    <!--- Definition: cfinvoke with component and method --->
    <cfinvoke component="models.Widget" method="render" returnvariable="invokeResult">

    <!--- Definition: cfinvoke with dotted path --->
    <cfinvoke component="services.UserService" method="listUsers" returnvariable="users">

</cfcomponent>
