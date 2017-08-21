angular.module('scionApp')
    .factory('adminService', ["$http", "$q", function($http, $q) {
    var adminService = {
        // Go to the user's home page
        me: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.get('/api/me').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        // Create SCIONLab VM
        scionLabVM: function(user) {
            // $http returns a promise, which has a then function, which also returns a promise
            // TODO(ercanucan): compose the URL in a cleaner fashion
            var url = '/api/as/generateVM?scionLabVMIP=' + user.scionLabVMIP + "&userEmail=" + user.Email;
            return $http.post(url).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
    };
    return adminService;
}]);
