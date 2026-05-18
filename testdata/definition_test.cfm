<cfset userService = getService("UserService")>
<cfset widgetService = getService("WidgetService")>

<!--- Definition: method on resolver-assigned variable --->
<cfset users = userService.listUsers()>
<cfset widget = widgetService.createWidget("demo")>

<!--- Definition: method on new expression --->
<cfset user = new models.User(1, "demo", "demo@test.com")>
<cfset displayName = user.getDisplayName()>

<!--- Definition: cfinvoke --->
<cfinvoke component="services.UserService" method="getUser" returnvariable="foundUser">
    <cfinvokeargument name="id" value="1">
</cfinvoke>

<!--- Definition: method on createObject --->
<cfset base = createObject("component", "models.Base")>
<cfset className = base.getClassName()>
