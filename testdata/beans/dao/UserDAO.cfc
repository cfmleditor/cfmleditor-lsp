component {

    function getAll() {
        return queryExecute("SELECT * FROM users");
    }

    function getById(required numeric id) {
        return queryExecute("SELECT * FROM users WHERE id = :id", {id: arguments.id});
    }

}
