angular.module('scionApp')
    .factory('userService', ["$http", "$q", function($http, $q) {
    var userService = {
        // Load the user's data
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
        generateSCIONLabVM: function(user) {
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
        // Remove SCIONLab VM
        removeSCIONLabVM: function(user) {
            // $http returns a promise, which has a then function, which also returns a promise
            console.log("Inside remove VM");
            var url = '/api/as/removeVM?userEmail=' + user.Email;
            return $http.post(url).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        }
    };
    return userService;
}]);
