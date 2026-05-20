component {

    property name="userDAO" inject="UserDAO@dao";
    property name="orderDAO" inject="orderDAO";
    property name="logger" type="services.BeanUserService";
    property name="config" type="string";
    property name="beanUserService" inject="BeanUserService@services";

    function init() {
        return this;
    }

    function processOrder(required numeric userId) {
        var user = variables.userDAO.getById(arguments.userId);
        var orders = variables.orderDAO.getByUserId(arguments.userId);
        return {user: user, orders: orders};
    }

}
