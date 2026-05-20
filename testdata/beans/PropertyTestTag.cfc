<cfcomponent>
    <cfproperty name="userDAO" type="string" inject="UserDAO@dao" />
    <cfproperty name="orderDAO" inject="orderDAO" />
    <cfproperty name="helper" type="services.BeanUserService" />

    <cffunction name="init" returntype="any">
        <cfreturn this />
    </cffunction>

    <cffunction name="doWork" returntype="void">
        <cfset var result = variables.userDAO.getById(1) />
    </cffunction>
</cfcomponent>
