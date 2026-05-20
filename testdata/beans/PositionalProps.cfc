component {

    property string name;
    property numeric age;
    property UserDAO userDAO;

    function init(required string name, numeric age = 0) {
        variables.name = arguments.name;
        variables.age = arguments.age;
        return this;
    }

    function getName() {
        return variables.name;
    }

}
