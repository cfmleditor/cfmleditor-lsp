<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>CFML LSP Comparison</title>
</head>
<body>
    <!--- Tag completion: type < to see cfml tag suggestions --->
    <cfoutput>
        <!--- Attribute completion: cursor inside a tag after space --->
        <cfquery name="getUsers" datasource="mydb">
            SELECT username, email FROM users WHERE active = 1
        </cfquery>

        <!--- Closing tag completion: type </ to see suggestions --->
        <cfloop query="getUsers">
            <p>#getUsers.username# - #getUsers.email#</p>
        </cfloop>

        <!--- Nested tags --->
        <cftry>
            <cfhttp method="GET" url="https://api.example.com/data">
                <cfhttpparam type="header" name="Accept" value="application/json">
            </cfhttp>
            <cfcatch type="any">
                <p>Error: #cfcatch.message#</p>
            </cfcatch>
        </cftry>

        <!--- Void/self-closing tags (no closing tag needed) --->
        <cfset greeting = "Hello">
        <cfparam name="username" default="Guest">
        <cfinclude template="header.cfm">

        <!--- Conditional structure: cfif/cfelseif/cfelse --->
        <cfif getUsers.recordCount GT 0>
            <p>Found #getUsers.recordCount# users</p>
        <cfelseif isDefined("fallbackList")>
            <p>Using fallback list</p>
        <cfelse>
            <p>No users found</p>
        </cfif>

        <!--- Expression tags (function completion inside) --->
        <cfset result = arrayLen(myArray)>
        <cfif len(trim(username)) GT 0>
            <p>Welcome, #username#!</p>
        </cfif>
    </cfoutput>
</body>
</html>
