<cfset pageTitle = "Product List">
<cfparam name="url.page" default="1" type="numeric">
<cfparam name="url.search" default="" type="string">

<cfinclude template="includes/header.cfm">

<cfoutput>
<h1>#pageTitle#</h1>

<cfif len(url.search)>
    <p>Showing results for: <strong>#encodeForHTML(url.search)#</strong></p>
</cfif>
</cfoutput>

<cfset productDAO = createObject("component", "ProductDAO").init("myDatasource")>
<cfset userService = createObject("component", "UserService").init("myDatasource")>

<cfset users = userService.getUsers(maxRows=10)>

<cfoutput>
<table>
    <thead>
        <tr>
            <th>ID</th>
            <th>Username</th>
            <th>Email</th>
        </tr>
    </thead>
    <tbody>
        <cfloop query="users">
            <tr>
                <td>#users.id#</td>
                <td>#encodeForHTML(users.username)#</td>
                <td>#encodeForHTML(users.email)#</td>
            </tr>
        </cfloop>
    </tbody>
</table>
</cfoutput>

<cfif url.page GT 1>
    <a href="index.cfm?page=#url.page - 1#">Previous</a>
</cfif>
<a href="index.cfm?page=#url.page + 1#">Next</a>

<cfinclude template="includes/footer.cfm">
