component extends="models.Base" {

    property name="id" type="numeric";
    property name="username" type="string";
    property name="email" type="string";

    function init(required numeric id, required string username, required string email) {
        variables.id = arguments.id;
        variables.username = arguments.username;
        variables.email = arguments.email;
        return this;
    }

    function getDisplayName() {
        return variables.username & " <" & variables.email & ">";
    }

    function toStruct() {
        return { id: variables.id, username: variables.username, email: variables.email };
    }

}
