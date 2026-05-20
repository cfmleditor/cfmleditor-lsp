component {

    function getByUserId(required numeric userId) {
        return queryExecute("SELECT * FROM orders WHERE user_id = :uid", {uid: arguments.userId});
    }

}
