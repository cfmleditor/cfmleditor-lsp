component accessors="true" {

    property name="datasource" type="string";
    property name="tableName" type="string" default="users";

    public UserService function init(required string datasource) {
        variables.datasource = arguments.datasource;
        return this;
    }

    public query function getUsers(numeric maxRows = 100) {
        var result = queryExecute(
            "SELECT id, username, email FROM #variables.tableName# LIMIT :maxRows",
            { maxRows: { value: arguments.maxRows, cfsqltype: "cf_sql_integer" } },
            { datasource: variables.datasource }
        );
        return result;
    }

    public struct function getUserById(required numeric id) {
        var qry = queryExecute(
            "SELECT id, username, email FROM users WHERE id = :id",
            { id: { value: arguments.id, cfsqltype: "cf_sql_integer" } },
            { datasource: variables.datasource }
        );
        if (qry.recordCount == 0) {
            throw(type="UserNotFound", message="User #arguments.id# not found");
        }
        return {
            id: qry.id,
            username: qry.username,
            email: qry.email
        };
    }

    private void function logAccess(required string action) {
        writeLog(text="UserService: #arguments.action#", type="information");
    }

}
