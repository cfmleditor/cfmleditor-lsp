component extends="models.Widget" {

    function doStuff() {
        // Line 3: unqualified call to inherited method from Widget
        var html = render();
        // Line 5: unqualified call to inherited method from Base (grandparent)
        var name = getClassName();
        // Line 7: super.method call
        var s = super.render();
        // Line 9: qualified call via typed argument
        return this;
    }

    function doMore(required models.Widget widget) {
        // Line 14: qualified call on typed argument — method in Widget
        var html = arguments.widget.render();
        // Line 16: qualified call on typed argument — method inherited from Base
        var name = arguments.widget.getClassName();
    }

}
