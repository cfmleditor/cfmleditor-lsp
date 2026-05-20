component {

    property name="userDAO" inject="UserDAO@dao";

    function getUser(required numeric id) {
        return variables.userDAO.getById(arguments.id);
    }

    function listUsers() {
        return variables.userDAO.getAll();
    }

}
