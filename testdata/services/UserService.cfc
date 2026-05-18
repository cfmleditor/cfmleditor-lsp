component {

    function init() {
        variables.users = {};
        return this;
    }

    function getUser(required numeric id) {
        return variables.users[arguments.id];
    }

    function createUser(required numeric id, required string username, required string email) {
        var user = new models.User(arguments.id, arguments.username, arguments.email);
        variables.users[arguments.id] = user;
        return user;
    }

    function listUsers() {
        return variables.users;
    }

}
