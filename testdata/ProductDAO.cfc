<cfcomponent displayname="ProductDAO" output="false">

    <cfproperty name="datasource" type="string">

    <cffunction name="init" access="public" returntype="ProductDAO">
        <cfargument name="datasource" type="string" required="true">
        <cfset variables.datasource = arguments.datasource>
        <cfreturn this>
    </cffunction>

    <cffunction name="getProduct" access="public" returntype="query">
        <cfargument name="productId" type="numeric" required="true">
        <cfargument name="includeInactive" type="boolean" required="false" default="false">

        <cfset var qProduct = "">

        <cfquery name="qProduct" datasource="#variables.datasource#">
            SELECT id, name, price, active
            FROM products
            WHERE id = <cfqueryparam value="#arguments.productId#" cfsqltype="cf_sql_integer">
            <cfif NOT arguments.includeInactive>
                AND active = 1
            </cfif>
        </cfquery>

        <cfreturn qProduct>
    </cffunction>

    <cffunction name="saveProduct" access="public" returntype="void">
        <cfargument name="name" type="string" required="true">
        <cfargument name="price" type="numeric" required="true">

        <cfquery datasource="#variables.datasource#">
            INSERT INTO products (name, price, active)
            VALUES (
                <cfqueryparam value="#arguments.name#" cfsqltype="cf_sql_varchar">,
                <cfqueryparam value="#arguments.price#" cfsqltype="cf_sql_decimal">,
                1
            )
        </cfquery>
    </cffunction>

    <cffunction name="deleteProduct" access="private" returntype="void">
        <cfargument name="productId" type="numeric" required="true">

        <cftry>
            <cfquery datasource="#variables.datasource#">
                DELETE FROM products WHERE id = <cfqueryparam value="#arguments.productId#" cfsqltype="cf_sql_integer">
            </cfquery>
            <cfcatch type="database">
                <cflog text="Failed to delete product #arguments.productId#: #cfcatch.message#" type="error">
            </cfcatch>
        </cftry>
    </cffunction>

    

</cfcomponent>
